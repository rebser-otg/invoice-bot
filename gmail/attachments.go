package gmail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
)

// Attachment is a single invoice file extracted from an email's MIME tree.
type Attachment struct {
	Filename string
	MimeType string // normalisiert auf einen vom Office-Hub akzeptierten Typ
	Data     []byte
}

// ExtractInvoiceAttachments walkt den MIME-Baum der rohen E-Mail und gibt alle
// Anhänge zurück, die der Spesen-Intake akzeptiert (PDF / JPEG / PNG / HEIC).
// Inline-Text, HTML-Bodies und sonstige Parts werden ignoriert. Leeres Ergebnis
// (kein passender Anhang) ist KEIN Fehler — der Aufrufer entscheidet, was dann
// passiert.
func ExtractInvoiceAttachments(raw []byte) ([]Attachment, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parsing message: %w", err)
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil {
		// Kein/ungültiger Content-Type → kein Multipart, also keine Anhänge.
		return nil, nil
	}
	if !strings.HasPrefix(mediaType, "multipart/") {
		return nil, nil
	}
	var out []Attachment
	if err := walkMultipart(msg.Body, params["boundary"], &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SelectInvoices wählt aus den extrahierten Anhängen die tatsächlichen
// Rechnungen aus. Hintergrund: kommt mehr als ein Anhang, ist der Zusatz
// meist eine Quittung/ein Beleg zur eigentlichen Rechnung — dann nur die
// Anhänge mit "invoice" im Dateinamen behalten.
//
//   - 0 oder 1 Anhang → unverändert (Einzel-Anhang wird immer genommen,
//     egal wie er heißt).
//   - mehrere, ≥1 mit "invoice" im Namen → nur die "invoice"-Anhänge.
//   - mehrere, KEINER mit "invoice" im Namen → alle behalten. Lieber ein
//     Beleg-Draft zu viel (im Portal löschbar) als eine Rechnung verpasst.
func SelectInvoices(atts []Attachment) []Attachment {
	if len(atts) <= 1 {
		return atts
	}
	var named []Attachment
	for _, a := range atts {
		if strings.Contains(strings.ToLower(a.Filename), "invoice") {
			named = append(named, a)
		}
	}
	if len(named) > 0 {
		return named
	}
	return atts
}

// walkMultipart liest einen multipart-Body und sammelt Invoice-Anhänge,
// rekursiv über verschachtelte multipart-Parts (z.B. multipart/mixed →
// multipart/alternative + application/pdf).
func walkMultipart(body io.Reader, boundary string, out *[]Attachment) error {
	if boundary == "" {
		return nil
	}
	mr := multipart.NewReader(body, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading multipart: %w", err)
		}

		mediaType, params, _ := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if strings.HasPrefix(strings.ToLower(mediaType), "multipart/") {
			if err := walkMultipart(part, params["boundary"], out); err != nil {
				part.Close()
				return err
			}
			part.Close()
			continue
		}

		filename := part.FileName() // dekodiert RFC2047-Header
		normMime, ok := invoiceMimeType(mediaType, filename)
		if !ok {
			part.Close()
			continue
		}
		data, err := decodePart(part)
		part.Close()
		if err != nil {
			return fmt.Errorf("decoding attachment %q: %w", filename, err)
		}
		if filename == "" {
			filename = "beleg"
		}
		*out = append(*out, Attachment{Filename: filename, MimeType: normMime, Data: data})
	}
	return nil
}

// invoiceMimeType prüft, ob ein Part ein vom Office-Hub akzeptierter Beleg ist,
// primär über den Content-Type, mit Datei-Endung als Fallback (manche Mailer
// schicken application/octet-stream für PDFs). Gibt den normalisierten Typ
// zurück.
func invoiceMimeType(mediaType, filename string) (string, bool) {
	switch strings.ToLower(mediaType) {
	case "application/pdf":
		return "application/pdf", true
	case "image/jpeg":
		return "image/jpeg", true
	case "image/png":
		return "image/png", true
	case "image/heic", "image/heif":
		return "image/heic", true
	}
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf", true
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg", true
	case strings.HasSuffix(lower, ".png"):
		return "image/png", true
	case strings.HasSuffix(lower, ".heic"), strings.HasSuffix(lower, ".heif"):
		return "image/heic", true
	}
	return "", false
}

// decodePart liest einen Part und dekodiert ihn gemäß
// Content-Transfer-Encoding. multipart.Reader dekodiert CTE NICHT selbst.
func decodePart(part *multipart.Part) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(part.Header.Get("Content-Transfer-Encoding"))) {
	case "base64":
		rawB64, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		// E-Mails brechen base64 in Zeilen um — Whitespace vor dem Decode raus.
		clean := strings.Map(func(r rune) rune {
			if r == '\r' || r == '\n' || r == ' ' || r == '\t' {
				return -1
			}
			return r
		}, string(rawB64))
		return base64.StdEncoding.DecodeString(clean)
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(part))
	default:
		return io.ReadAll(part)
	}
}
