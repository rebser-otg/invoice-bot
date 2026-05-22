package forwarder

import (
	"fmt"
	"os"

	"github.com/rebser-otg/invoice-bot/config"
	"github.com/rebser-otg/invoice-bot/gmail"
	"github.com/rebser-otg/invoice-bot/memory"
)

// MailClient is the interface the forwarder uses to interact with Gmail.
// *gmail.Client satisfies this interface.
type MailClient interface {
	Search(senders []string) ([]string, error)
	FetchRaw(id string) ([]byte, error)
	Send(raw []byte) error
}

// Result holds the outcome of a Run call.
type Result struct {
	Forwarded   int
	Failed      int
	AlreadySeen int
}

// Run searches for invoice emails, filters out already-forwarded ones,
// forwards new ones via client, and updates mem.
// mem is NOT saved to disk — the caller is responsible for calling mem.Save.
func Run(cfg *config.Config, mem *memory.Memory, client MailClient) (Result, error) {
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

		fwd, err := gmail.BuildForward(raw, cfg.ForwardTo, cfg.MessageText)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [error] building forward for %s: %v\n", id, err)
			res.Failed++
			continue
		}

		if err := client.Send(fwd); err != nil {
			fmt.Fprintf(os.Stderr, "  [error] sending %s: %v\n", id, err)
			res.Failed++
			continue
		}

		mem.Add(id)
		res.Forwarded++
	}

	return res, nil
}
