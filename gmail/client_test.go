package gmail_test

import (
	"path/filepath"
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
}
