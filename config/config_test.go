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

const validYAML = "api_base_url: https://offices.onetech.group\napi_token: otg_testtoken\n"

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", validYAML)
	writeFile(t, dir, "senders.txt", "# comment\nbilling@anthropic.com\n\ninvoices@openai.com\n")

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIBaseURL != "https://offices.onetech.group" {
		t.Errorf("APIBaseURL = %q", cfg.APIBaseURL)
	}
	if cfg.APIToken != "otg_testtoken" {
		t.Errorf("APIToken = %q", cfg.APIToken)
	}
	if len(cfg.Senders) != 2 {
		t.Fatalf("Senders len = %d, want 2", len(cfg.Senders))
	}
	if cfg.Senders[0] != "billing@anthropic.com" || cfg.Senders[1] != "invoices@openai.com" {
		t.Errorf("Senders = %v", cfg.Senders)
	}
}

func TestLoad_TrimsTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "api_base_url: https://offices.onetech.group/\napi_token: otg_x\n")
	writeFile(t, dir, "senders.txt", "billing@anthropic.com\n")
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIBaseURL != "https://offices.onetech.group" {
		t.Errorf("trailing slash not trimmed: %q", cfg.APIBaseURL)
	}
}

func TestLoad_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "config.yaml") {
		t.Errorf("expected config.yaml error, got: %v", err)
	}
}

func TestLoad_MissingSenders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", validYAML)
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "senders.txt") {
		t.Errorf("expected senders.txt error, got: %v", err)
	}
}

func TestLoad_MissingBaseURL(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "api_token: otg_x\n")
	writeFile(t, dir, "senders.txt", "billing@anthropic.com\n")
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "api_base_url") {
		t.Errorf("expected api_base_url error, got: %v", err)
	}
}

func TestLoad_MissingToken(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", "api_base_url: https://offices.onetech.group\n")
	writeFile(t, dir, "senders.txt", "billing@anthropic.com\n")
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "api_token") {
		t.Errorf("expected api_token error, got: %v", err)
	}
}

func TestLoad_EmptySenders(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config.yaml", validYAML)
	writeFile(t, dir, "senders.txt", "# only comments\n\n")
	_, err := config.Load(dir)
	if err == nil || !strings.Contains(err.Error(), "sender") {
		t.Errorf("expected sender error, got: %v", err)
	}
}
