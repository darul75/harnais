package imapmail

import (
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
)

func TestParseMessage(t *testing.T) {

	raw := "From: Jean Dupont <jean@example.com>\r\n" +
		"Subject: =?UTF-8?B?UGxhbiBkZSB0cmF2YWls?=\r\n" +
		"Date: Mon, 02 Sep 2026 14:00:00 +0200\r\n" +
		"Message-ID: <abc123@mail.example.com>\r\n" +
		"X-Gm-Msgid: 1900000000000001\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		"Bonjour, ceci est un test.\r\n"

	envelope := &imap.Envelope{
		Date:      time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC),
		Subject:   "Plan de travail",
		MessageID: "abc123@mail.example.com",
		From: []imap.Address{
			{
				Name:    "Jean Dupont",
				Mailbox: "jean",
				Host:    "example.com",
			},
		},
	}

	email, err :=
		parseMessage(
			envelope,
			[]byte(raw),
		)

	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}

	if email.Subject != "Plan de travail" {
		t.Errorf("subject = %q, want %q", email.Subject, "Plan de travail")
	}

	if email.Sender != "Jean Dupont <jean@example.com>" {
		t.Errorf("sender = %q, want %q", email.Sender, "Jean Dupont <jean@example.com>")
	}

	if email.GmailID != "1900000000000001" {
		t.Errorf("gmailID = %q, want %q", email.GmailID, "1900000000000001")
	}

	if email.MessageID != "abc123@mail.example.com" {
		t.Errorf("messageID = %q, want %q", email.MessageID, "abc123@mail.example.com")
	}

	expectedLink := "https://mail.google.com/mail/u/0/#inbox/1900000000000001"
	if email.Link != expectedLink {
		t.Errorf("link = %q, want %q", email.Link, expectedLink)
	}

	if email.Body != "Bonjour, ceci est un test." {
		t.Errorf("body = %q, want %q", email.Body, "Bonjour, ceci est un test.")
	}
}

func TestParseMessageNoSubject(t *testing.T) {

	raw := "From: nobody@example.com\r\n" +
		"Date: Tue, 03 Sep 2026 10:00:00 +0200\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"No subject header here.\r\n"

	envelope := &imap.Envelope{
		Date:      time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		MessageID: "",
		From: []imap.Address{
			{
				Name:    "",
				Mailbox: "nobody",
				Host:    "example.com",
			},
		},
	}

	email, err :=
		parseMessage(
			envelope,
			[]byte(raw),
		)

	if err != nil {
		t.Fatalf("parseMessage: %v", err)
	}

	if email.Subject != "(no subject)" {
		t.Errorf("subject = %q, want %q", email.Subject, "(no subject)")
	}

	if email.Link != "" {
		t.Errorf("link = %q, want empty", email.Link)
	}
}

func TestLinkURL(t *testing.T) {

	tests := []struct {
		name      string
		gmailID   string
		messageID string
		want      string
	}{
		{
			name:    "gmail id",
			gmailID: "1900000000000001",
			want:    "https://mail.google.com/mail/u/0/#inbox/1900000000000001",
		},
		{
			name:      "message id fallback",
			gmailID:   "",
			messageID: "abc123@example.com",
			want:      "https://mail.google.com/mail/u/0/#search/rfc822msgid:abc123@example.com",
		},
		{
			name:    "both empty",
			gmailID: "",
			want:    "",
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {

			got :=
				LinkURL(
					tc.gmailID,
					tc.messageID,
				)

			if got != tc.want {
				t.Errorf("LinkURL(%q, %q) = %q, want %q",
					tc.gmailID, tc.messageID, got, tc.want)
			}
		})
	}
}