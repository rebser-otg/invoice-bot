package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rebser-otg/invoice-bot/config"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "forward_to: test@example.com\n")
	writeFile(t, dir, "senders.txt", "# comment\nbilling@anthropic.com\n\ninvoices@openai.com\n")
	writeFile(t, dir, "message.txt", "Please forward this invoice.\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ForwardTo != "test@example.com" {
		t.Errorf("ForwardTo = %q, want %q", cfg.ForwardTo, "test@example.com")
	}
	if len(cfg.Senders) != 2 {
		t.Fatalf("Senders len = %d, want 2", len(cfg.Senders))
	}
	if cfg.Senders[0] != "billing@anthropic.com" {
		t.Errorf("Senders[0] = %q", cfg.Senders[0])
	}
	if cfg.Senders[1] != "invoices@openai.com" {
		t.Errorf("Senders[1] = %q", cfg.Senders[1])
	}
	if cfg.MessageText != "Please forward this invoice.\n" {
		t.Errorf("MessageText = %q", cfg.MessageText)
	}
}

func TestLoad_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for missing config.yaml")
	}
}

func TestLoad_MissingSenders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "forward_to: test@example.com\n")
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for missing senders.txt")
	}
}

func TestLoad_MissingMessage(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "forward_to: test@example.com\n")
	writeFile(t, dir, "senders.txt", "billing@anthropic.com\n")
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for missing message.txt")
	}
}

func TestLoad_EmptyForwardTo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "forward_to: \"\"\n")
	writeFile(t, dir, "senders.txt", "billing@anthropic.com\n")
	writeFile(t, dir, "message.txt", "msg\n")
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for empty forward_to")
	}
	if !strings.Contains(err.Error(), "forward_to") {
		t.Errorf("error should mention forward_to, got: %v", err)
	}
}

func TestLoad_EmptySenders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "forward_to: test@example.com\n")
	writeFile(t, dir, "senders.txt", "# only comments\n\n")
	writeFile(t, dir, "message.txt", "msg\n")
	_, err := config.Load(dir)
	if err == nil {
		t.Fatal("expected error for empty senders list")
	}
	if !strings.Contains(err.Error(), "sender") {
		t.Errorf("error should mention sender, got: %v", err)
	}
}
