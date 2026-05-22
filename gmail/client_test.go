package gmail_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/rebser-otg/invoice-bot/gmail"
)

func TestNewClient_MissingCredentials(t *testing.T) {
	dir := t.TempDir()
	_, err := gmail.NewClient(
		filepath.Join(dir, "credentials.json"),
		filepath.Join(dir, "token.json"),
	)
	if err == nil {
		t.Fatal("expected error when credentials.json is missing")
	}
	if !strings.Contains(err.Error(), "console.cloud.google.com") {
		t.Errorf("error should contain actionable URL, got: %v", err)
	}
}
