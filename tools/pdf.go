package tools

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

// MaxPDFTextBytes caps how much extracted text a PDF reader returns
// so a very large document does not blow past LLM context windows.
const MaxPDFTextBytes = 100000

// ExtractPDFText reads the text content of a PDF file at path.
// Scanned (image-only) PDFs yield little or no text.
func ExtractPDFText(
	path string,
) (string, error) {

	file, reader, err :=
		pdf.Open(path)

	if err != nil {
		return "", fmt.Errorf(
			"pdf open: %w",
			err,
		)
	}

	defer file.Close()

	var builder strings.Builder

	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {

		pageText, err :=
			reader.Page(pageNum).GetPlainText(nil)

		if err != nil {
			continue
		}

		builder.WriteString(pageText)

		if builder.Len() >= MaxPDFTextBytes {
			break
		}
	}

	text :=
		strings.TrimSpace(
			builder.String(),
		)

	if text == "" {
		return "", fmt.Errorf(
			"no extractable text found (scanned or image-only PDF?)",
		)
	}

	// Trim to the cap on a rune-safe boundary to avoid splitting
	// a multi-byte rune when we cut.
	return runeSafeCut(
		text,
		MaxPDFTextBytes,
	), nil
}

func runeSafeCut(
	value string,
	maxBytes int,
) string {

	if len(value) <= maxBytes {
		return value
	}

	cut := value[:maxBytes]

	for len(cut) > 0 && !isValidUTF8Tail(cut) {
		cut = cut[:len(cut)-1]
	}

	return cut
}

func isValidUTF8Tail(
	value string,
) bool {

	return utf8.ValidString(value)
}