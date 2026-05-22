package gmail_test

import (
	"net/mail"
	"strings"
	"testing"

	"github.com/rebser-otg/invoice-bot/gmail"
)

var simpleRaw = []byte(
	"From: billing@anthropic.com\r\n" +
		"To: robin@example.com\r\n" +
		"Subject: Your Invoice #123\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain\r\n" +
		"\r\n" +
		"Invoice body here.",
)

func TestBuildForward_Subject(t *testing.T) {
	result, err := gmail.BuildForward(simpleRaw, "fwd@example.com", "Please see the invoice.\n\n---\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(result)))
	if err != nil {
		t.Fatalf("result is not valid RFC 2822: %v", err)
	}
	if got := msg.Header.Get("Subject"); got != "Fwd: Your Invoice #123" {
		t.Errorf("Subject = %q, want %q", got, "Fwd: Your Invoice #123")
	}
}

func TestBuildForward_To(t *testing.T) {
	result, err := gmail.BuildForward(simpleRaw, "fwd@example.com", "Note:\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg, _ := mail.ReadMessage(strings.NewReader(string(result)))
	if got := msg.Header.Get("To"); got != "fwd@example.com" {
		t.Errorf("To = %q, want %q", got, "fwd@example.com")
	}
}

func TestBuildForward_AlreadyFwdPrefix(t *testing.T) {
	raw := []byte(
		"From: billing@anthropic.com\r\n" +
			"Subject: Fwd: Old Invoice\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\n" +
			"body",
	)
	result, err := gmail.BuildForward(raw, "fwd@example.com", "Note:\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg, _ := mail.ReadMessage(strings.NewReader(string(result)))
	// Should not double-prefix
	if got := msg.Header.Get("Subject"); got != "Fwd: Old Invoice" {
		t.Errorf("Subject = %q, want %q", got, "Fwd: Old Invoice")
	}
}

func TestBuildForward_IsValidMultipart(t *testing.T) {
	result, err := gmail.BuildForward(simpleRaw, "fwd@example.com", "Note:\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msg, err := mail.ReadMessage(strings.NewReader(string(result)))
	if err != nil {
		t.Fatalf("result is not valid RFC 2822: %v", err)
	}
	ct := msg.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/mixed") {
		t.Errorf("Content-Type = %q, want multipart/mixed", ct)
	}
}
