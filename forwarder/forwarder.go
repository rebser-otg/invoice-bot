package forwarder

import (
	"fmt"
	"os"

	"github.com/rebser-otg/invoice-bot/config"
	"github.com/rebser-otg/invoice-bot/gmail"
	"github.com/rebser-otg/invoice-bot/memory"
)

// MailClient is the interface the forwarder uses to read Gmail.
// *gmail.Client satisfies this interface.
type MailClient interface {
	Search(senders []string) ([]string, error)
	FetchRaw(id string) ([]byte, error)
}

// Uploader hands a single invoice attachment to the Office-Hub intake.
// *uploader.Uploader satisfies this interface.
type Uploader interface {
	Upload(att gmail.Attachment) error
}

// Result holds the outcome of a Run call.
type Result struct {
	Uploaded    int // E-Mails mit ≥1 erfolgreich hochgeladenem Anhang
	Skipped     int // E-Mails ohne passenden Anhang (als gesehen markiert)
	Failed      int // Fetch-/Upload-Fehler (NICHT als gesehen markiert → Retry)
	AlreadySeen int
}

// Run searches for invoice emails, extracts their PDF/image attachments, and
// uploads each to the Office-Hub intake. Already-seen emails are skipped.
// mem is NOT saved to disk — the caller is responsible for calling mem.Save.
//
// Retry-Semantik: ein Email wird erst nach erfolgreichem Hand-in als gesehen
// markiert. Bei Upload-Fehler bleibt es ungesehen und wird beim nächsten Run
// erneut versucht — der Intake dedupt identische Belege per Hash, ein
// Re-Upload bereits eingereichter Anhänge legt also keinen Doppel-Entwurf an.
func Run(cfg *config.Config, mem *memory.Memory, client MailClient, up Uploader) (Result, error) {
	ids, err := client.Search(cfg.Senders)
	if err != nil {
		return Result{}, fmt.Errorf("searching gmail: %w", err)
	}

	var res Result
	for _, id := range ids {
		if mem.Contains(id) {
			res.AlreadySeen++
			continue
		}

		raw, err := client.FetchRaw(id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [error] fetching %s: %v\n", id, err)
			res.Failed++
			continue
		}

		atts, err := gmail.ExtractInvoiceAttachments(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [error] extracting attachments from %s: %v\n", id, err)
			res.Failed++
			continue
		}

		// Bei mehreren Anhängen nur die eigentliche(n) Rechnung(en) nehmen —
		// Zusatz-PDFs sind meist Quittungen/Belege.
		atts = gmail.SelectInvoices(atts)

		if len(atts) == 0 {
			// Kein PDF/Bild im Mail (Invoice inline oder als Download-Link).
			// Als gesehen markieren, damit es nicht jeden Run erneut auftaucht.
			fmt.Fprintf(os.Stderr, "  [skip] %s: kein PDF/Bild-Anhang — übersprungen\n", id)
			mem.Add(id)
			res.Skipped++
			continue
		}

		if err := uploadAll(up, id, atts); err != nil {
			fmt.Fprintf(os.Stderr, "  [error] uploading %s: %v\n", id, err)
			res.Failed++
			continue
		}

		mem.Add(id)
		res.Uploaded++
	}

	return res, nil
}

// uploadAll lädt alle Anhänge eines Mails hoch. Schlägt einer fehl, bricht es
// ab und gibt den Fehler zurück — das Mail bleibt ungesehen und der ganze
// Satz wird beim nächsten Run erneut versucht (Hash-Dedup verhindert Dubletten
// für die bereits erfolgreichen).
func uploadAll(up Uploader, id string, atts []gmail.Attachment) error {
	for i, att := range atts {
		if err := up.Upload(att); err != nil {
			return fmt.Errorf("attachment %d/%d (%s): %w", i+1, len(atts), att.Filename, err)
		}
	}
	return nil
}
