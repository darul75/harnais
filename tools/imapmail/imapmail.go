// Package imapmail reads email messages from an IMAP account,
// returning lightweight, digest-ready summaries of recent messages.
package imapmail

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// Config holds the connection and search settings for an IMAP account.
type Config struct {
	// Email is the account address used to log in.
	Email string

	// AppPassword is the account's app-specific password.
	AppPassword string

	// Host is the IMAP server host, e.g. "imap.gmail.com".
	Host string

	// Port is the IMAP server port, e.g. "993".
	Port string

	// Mailbox is the folder to read, defaulting to "INBOX".
	Mailbox string

	// DaysBack is how many days of messages to fetch. Zero means today.
	DaysBack int

	// MaxMessages caps how many messages are returned.
	MaxMessages int
}

// Email is a digest-ready summary of a single message.
type Email struct {
	Sender  string
	Subject string
	Date    time.Time
	Body    string
	// GmailID is Gmail's per-message ID (X-GM-MSGID), when available.
	GmailID string
	// MessageID is the RFC 5322 Message-ID header, without angle brackets.
	MessageID string
	// Link is the Gmail URL that opens the exact message.
	Link string
}

// LinkURL builds the Gmail URL that opens a message. It prefers Gmail's
// X-GM-MSGID, falling back to an rfc822msgid search from Message-ID.
func LinkURL(gmailID, messageID string) string {

	if gmailID != "" {
		return "https://mail.google.com/mail/u/0/#inbox/" + gmailID
	}

	if messageID != "" {
		return "https://mail.google.com/mail/u/0/#search/" + escapeFragment("rfc822msgid:"+messageID)
	}

	return ""
}

// escapeFragment percent-escapes a value for use inside a Gmail search
// URL fragment.
func escapeFragment(value string) string {

	replacer :=
		strings.NewReplacer(
			"%", "%25",
			" ", "%20",
			"<", "%3C",
			">", "%3E",
			"#", "%23",
			"/", "%2F",
			"&", "%26",
			"=", "%3D",
		)

	return replacer.Replace(value)
}

// FetchLatest connects to the account, searches the mailbox for messages
// from the last DaysBack days, and returns up to MaxMessages summaries.
// It returns an error if the account is not configured or the server
// cannot be reached.
func FetchLatest(
	ctx context.Context,
	cfg Config,
) ([]Email, error) {

	if cfg.Email == "" || cfg.AppPassword == "" {
		return nil, fmt.Errorf(
			"gmail account not configured",
		)
	}

	host := cfg.Host

	if host == "" {
		host = "imap.gmail.com"
	}

	port := cfg.Port

	if port == "" {
		port = "993"
	}

	mailbox := cfg.Mailbox

	if mailbox == "" {
		mailbox = "INBOX"
	}

	daysBack := cfg.DaysBack

	if daysBack < 0 {
		daysBack = 0
	}

	max := cfg.MaxMessages

	if max <= 0 {
		max = 100
	}

	client, err :=
		imapclient.DialTLS(
			host+":"+port,
			&imapclient.Options{},
		)

	if err != nil {
		return nil, fmt.Errorf(
			"imap connect: %w",
			err,
		)
	}

	defer client.Close()

	if err :=
		client.Login(
			cfg.Email,
			cfg.AppPassword,
		).Wait(); err != nil {

		return nil, fmt.Errorf(
			"imap login: %w",
			err,
		)
	}

	if _, err :=
		client.Select(
			mailbox,
			nil,
		).Wait(); err != nil {

		return nil, fmt.Errorf(
			"imap select %q: %w",
			mailbox,
			err,
		)
	}

	criteria :=
		&imap.SearchCriteria{
			Since: sinceDate(daysBack),
		}

	data, err :=
		client.UIDSearch(
			criteria,
			nil,
		).Wait()

	if err != nil {
		return nil, fmt.Errorf(
			"imap search: %w",
			err,
		)
	}

	uids :=
		data.AllUIDs()

	if len(uids) > max {
		// Take the most recent messages (highest UIDs).
		uids =
			uids[len(uids)-max:]
	}

	if len(uids) == 0 {
		return []Email{}, nil
	}

	// Fetch the whole message and parse it locally, so we get plain
	// text bodies, Message-ID, and Gmail's X-GM-MSGID in one pass.
	bodySection :=
		&imap.FetchItemBodySection{
			Peek: true,
		}

	buffers, err :=
		client.Fetch(
			imap.UIDSetNum(uids...),
			&imap.FetchOptions{
				UID:         true,
				Envelope:    true,
				BodySection: []*imap.FetchItemBodySection{bodySection},
			},
		).Collect()

	if err != nil {
		return nil, fmt.Errorf(
			"imap fetch: %w",
			err,
		)
	}

	emails :=
		make([]Email, 0, len(buffers))

	for _, buf := range buffers {

		email, parseErr :=
			parseMessage(
				buf.Envelope,
				buf.FindBodySection(
					bodySection,
				),
			)

		if parseErr != nil {
			continue
		}

		emails =
			append(
				emails,
				email,
			)
	}

	return emails, nil
}

// sinceDate returns the start of the window to search, at UTC midnight.
func sinceDate(daysBack int) time.Time {

	now :=
		time.Now()

	return time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		0,
		0,
		0,
		0,
		time.UTC,
	).AddDate(0, 0, -daysBack)
}

// parseMessage parses raw message bytes into an Email summary.
func parseMessage(
	envelope *imap.Envelope,
	raw []byte,
) (Email, error) {

	msg, err :=
		mail.ReadMessage(
			strings.NewReader(
				string(raw),
			),
		)

	if err != nil {
		return Email{}, fmt.Errorf(
			"parse message: %w",
			err,
		)
	}

	header :=
		msg.Header

	date, _ :=
		header.Date()

	gmailID :=
		strings.TrimSpace(
			header.Get("X-Gm-Msgid"),
		)

	if gmailID == "" {
		gmailID =
			strings.TrimSpace(
				header.Get("X-Gm-Messageid"),
			)
	}

	subject :=
		decodeHeader(
			header.Get("Subject"),
		)

	messageID :=
		strings.Trim(
			header.Get("Message-Id"),
			"<> \t",
		)

	sender :=
		decodeHeader(
			header.Get("From"),
		)

	// Prefer the envelope values when the raw parse left us empty,
	// since the envelope is already decoded by the library.
	if envelope != nil {

		if subject == "" {
			subject =
				envelope.Subject
		}

		if messageID == "" {
			messageID =
				envelope.MessageID
		}

		if date.IsZero() {
			date =
				envelope.Date
		}

		if len(envelope.From) > 0 {
			sender =
				formatAddress(
					&envelope.From[0],
				)
		}
	}

	body :=
		readBody(
			textproto.MIMEHeader(header),
			msg.Body,
		)

	body =
		strings.TrimSpace(body)

	if strings.TrimSpace(subject) == "" {
		subject = "(no subject)"
	}

	if strings.TrimSpace(sender) == "" {
		sender = "unknown"
	}

	return Email{
		Sender:    sender,
		Subject:   subject,
		Date:      date,
		Body:      body,
		GmailID:   gmailID,
		MessageID: messageID,
		Link:      LinkURL(gmailID, messageID),
	}, nil
}

// formatAddress renders an address as "Name <mailbox@host>" when a name
// is present, or just the address otherwise.
func formatAddress(
	addr *imap.Address,
) string {

	full :=
		addr.Addr()

	if full == "" {
		return ""
	}

	if strings.TrimSpace(addr.Name) == "" {
		return full
	}

	return addr.Name + " <" + full + ">"
}

// readBody extracts the plain-text content of a message, handling
// simple bodies and multipart/alternative parts.
func readBody(
	header textproto.MIMEHeader,
	body io.Reader,
) string {

	mediaType, params, err :=
		mime.ParseMediaType(
			header.Get("Content-Type"),
		)

	if err != nil {
		mediaType = "text/plain"
	}

	switch {

	case strings.HasPrefix(
		mediaType,
		"text/plain",
	):

		data, _ :=
			io.ReadAll(body)

		return decodeBody(
			header.Get("Content-Transfer-Encoding"),
			data,
		)

	case strings.HasPrefix(
		mediaType,
		"text/html",
	):

		data, _ :=
			io.ReadAll(body)

		return stripHTML(
			decodeBody(
				header.Get("Content-Transfer-Encoding"),
				data,
			),
		)

	case strings.HasPrefix(
		mediaType,
		"multipart/",
	):

		mr :=
			multipart.NewReader(
				body,
				params["boundary"],
			)

		var text string

		for {

			part, partErr :=
				mr.NextPart()

			if partErr != nil {
				break
			}

			partHeader :=
				textproto.MIMEHeader(
					part.Header,
				)

			partMediaType, _, partParseErr :=
				mime.ParseMediaType(
					partHeader.Get("Content-Type"),
				)

			if partParseErr != nil {
				part.Close()

				continue
			}

			switch {

			case strings.HasPrefix(
				partMediaType,
				"text/plain",
			):

				data, _ :=
					io.ReadAll(part)

				partText :=
					decodeBody(
						partHeader.Get("Content-Transfer-Encoding"),
						data,
					)

				if strings.TrimSpace(partText) != "" {
					text =
						partText

					part.Close()

					return text
				}

			case strings.HasPrefix(
				partMediaType,
				"text/html",
			):

				if text == "" {

					data, _ :=
						io.ReadAll(part)

					htmlText :=
						stripHTML(
							decodeBody(
								partHeader.Get("Content-Transfer-Encoding"),
								data,
							),
						)

					if strings.TrimSpace(htmlText) != "" {
						text =
							htmlText
					}
				}
			}

			part.Close()
		}

		return text
	}

	return ""
}

// decodeHeader decodes an RFC 2047 encoded-word header value.
func decodeHeader(value string) string {

	decoder :=
		&mime.WordDecoder{}

	decoded, err :=
		decoder.DecodeHeader(value)

	if err != nil {
		return value
	}

	return decoded
}

// decodeBody applies quoted-printable or base64 decoding based on the
// content transfer encoding header.
func decodeBody(
	encoding string,
	data []byte,
) string {

	encoding =
		strings.ToLower(
			strings.TrimSpace(encoding),
		)

	content :=
		string(data)

	switch encoding {

	case "quoted-printable":

		decoded, err :=
			io.ReadAll(
				quotedprintable.NewReader(
					strings.NewReader(content),
				),
			)

		if err == nil {
			return string(decoded)
		}

	case "base64":

		stripped :=
			strings.Map(
				func(r rune) rune {
					if r == '\r' ||
						r == '\n' {
						return -1
					}

					return r
				},
				content,
			)

		decoded, err :=
			base64.StdEncoding.DecodeString(
				stripped,
			)

		if err == nil {
			return string(decoded)
		}
	}

	return content
}

// stripHTML removes tags and HTML entities from an HTML body so the
// LLM digest sees the bare text.
func stripHTML(value string) string {

	var builder strings.Builder

	inTag := false

	for _, r := range value {

		switch r {

		case '<':
			inTag = true

		case '>':

			inTag = false

			builder.WriteString(" ")

		default:

			if !inTag {
				builder.WriteRune(r)
			}
		}
	}

	text :=
		builder.String()

	text =
		strings.ReplaceAll(text, "&nbsp;", " ")
	text =
		strings.ReplaceAll(text, "&amp;", "&")
	text =
		strings.ReplaceAll(text, "&lt;", "<")
	text =
		strings.ReplaceAll(text, "&gt;", ">")
	text =
		strings.ReplaceAll(text, "&quot;", `"`)
	text =
		strings.ReplaceAll(text, "&#39;", "'")

	return strings.Join(
		strings.Fields(text),
		" ",
	)
}

// Check connects to the IMAP server, attempts a login, and closes.
// It returns nil on success so the Settings UI can confirm the
// credentials are valid.
func Check(
	ctx context.Context,
	cfg Config,
) error {

	host := cfg.Host

	if host == "" {
		host = "imap.gmail.com"
	}

	port := cfg.Port

	if port == "" {
		port = "993"
	}

	client, err :=
		imapclient.DialTLS(
			host+":"+port,
			&imapclient.Options{},
		)

	if err != nil {
		return fmt.Errorf(
			"imap connect: %w",
			err,
		)
	}

	defer client.Close()

	if err :=
		client.Login(
			cfg.Email,
			cfg.AppPassword,
		).Wait(); err != nil {

		return fmt.Errorf(
			"imap login: %w",
			err,
		)
	}

	return nil
}