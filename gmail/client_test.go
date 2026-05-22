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

func TestBuildQuery_Multiple(t *testing.T) {
	got := gmail.BuildQuery([]string{"a@example.com", "b@example.com"})
	want := "from:(a@example.com OR b@example.com)"
	if got != want {
		t.Errorf("BuildQuery = %q, want %q", got, want)
	}
}

func TestBuildQuery_Single(t *testing.T) {
	got := gmail.BuildQuery([]string{"a@example.com"})
	want := "from:(a@example.com)"
	if got != want {
		t.Errorf("BuildQuery = %q, want %q", got, want)
	}
}

// TestClient_ImplementsMailClient verifies Client satisfies the forwarder.MailClient interface at compile time.
func TestClient_ImplementsMailClient(t *testing.T) {
	var _ interface {
		Search([]string) ([]string, error)
		FetchRaw(string) ([]byte, error)
		Send([]byte) error
	} = (*gmail.Client)(nil)
}
