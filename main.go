package main

import (
	"fmt"
	"os"

	"github.com/rebser-otg/invoice-bot/config"
	"github.com/rebser-otg/invoice-bot/forwarder"
	"github.com/rebser-otg/invoice-bot/gmail"
	"github.com/rebser-otg/invoice-bot/memory"
)

func main() {
	// All files are expected relative to the current working directory.
	// Run this binary from the project root.
	cfg, err := config.Load(".")
	if err != nil {
		fatalf("loading config: %v", err)
	}

	const memPath = "memory.json"
	mem, err := memory.Load(memPath)
	if err != nil {
		fatalf("loading memory: %v", err)
	}

	client, err := gmail.NewClient("credentials.json", "token.json")
	if err != nil {
		fatalf("connecting to Gmail:\n%v", err)
	}

	result, err := forwarder.Run(cfg, mem, client)
	if err != nil {
		fatalf("running forwarder: %v", err)
	}

	if err := mem.Save(memPath); err != nil {
		fatalf("saving memory: %v", err)
	}

	if result.Forwarded == 0 && result.Failed == 0 {
		fmt.Println("No new invoices found.")
		return
	}
	fmt.Printf("Forwarded %d new invoice(s). %d failed. %d already seen.\n",
		result.Forwarded, result.Failed, result.AlreadySeen)
	if result.Failed > 0 {
		os.Exit(1)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
