// Package uploader reicht extrahierte Beleg-Anhänge beim Office-Hub-Spesen-
// Intake ein (POST /api/expenses/intake, Bearer-Auth). Stdlib-only.
package uploader

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"

	"github.com/rebser-otg/invoice-bot/gmail"
)

// Uploader posts invoice attachments to the Office-Hub intake endpoint.
type Uploader struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// New returns an Uploader with a sane default HTTP timeout.
func New(baseURL, token string) *Uploader {
	return &Uploader{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Upload posts one attachment as multipart/form-data. Erfolg = HTTP 200; alles
// andere ist ein Fehler (mit dem Response-Body als Kontext). Der Server legt
// einen Spesen-Entwurf an und dedupt identische Belege per Hash, daher ist ein
// Re-Upload nach einem fehlgeschlagenen Run unschädlich.
func (u *Uploader) Upload(att gmail.Attachment) error {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	// Eigener Part-Header, damit der echte Content-Type mitgeht (CreateFormFile
	// würde application/octet-stream setzen — der Server würde dann nur über die
	// Datei-Endung normalisieren).
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename=%q`, att.Filename))
	h.Set("Content-Type", att.MimeType)
	part, err := w.CreatePart(h)
	if err != nil {
		return fmt.Errorf("creating form part: %w", err)
	}
	if _, err := part.Write(att.Data); err != nil {
		return fmt.Errorf("writing file part: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	req, err := http.NewRequest("POST", u.BaseURL+"/api/expenses/intake", &body)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+u.Token)

	resp, err := u.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("posting to intake: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("intake returned %d: %s", resp.StatusCode, bytes.TrimSpace(snippet))
	}
	return nil
}
