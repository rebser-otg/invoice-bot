package gmail

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
)

// BuildForward constructs a forwarded email as RFC 2822 bytes.
// The result is a multipart/mixed message:
//   - Part 1: text/plain with messageText (from message.txt)
//   - Part 2: message/rfc822 with the original raw email
func BuildForward(rawOriginal []byte, forwardTo, messageText string) ([]byte, error) {
	origMsg, err := mail.ReadMessage(bytes.NewReader(rawOriginal))
	if err != nil {
		return nil, fmt.Errorf("parsing original message: %w", err)
	}

	subject := origMsg.Header.Get("Subject")
	dec := new(mime.WordDecoder)
	if decoded, err := dec.DecodeHeader(subject); err == nil {
		subject = decoded
	}
	if !strings.HasPrefix(subject, "Fwd: ") {
		subject = "Fwd: " + subject
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Write outer headers before any multipart boundary
	fmt.Fprintf(&buf, "To: %s\r\n", forwardTo)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", w.Boundary())

	// Part 1: the message.txt preamble
	hdr1 := textproto.MIMEHeader{}
	hdr1.Set("Content-Type", "text/plain; charset=utf-8")
	p1, err := w.CreatePart(hdr1)
	if err != nil {
		return nil, fmt.Errorf("creating text part: %w", err)
	}
	fmt.Fprint(p1, messageText)

	// Part 2: the original email as an RFC 2822 attachment
	hdr2 := textproto.MIMEHeader{}
	hdr2.Set("Content-Type", "message/rfc822")
	hdr2.Set("Content-Disposition", `attachment; filename="original.eml"`)
	p2, err := w.CreatePart(hdr2)
	if err != nil {
		return nil, fmt.Errorf("creating rfc822 part: %w", err)
	}
	if _, err := p2.Write(rawOriginal); err != nil {
		return nil, fmt.Errorf("writing original message: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}
	return buf.Bytes(), nil
}
