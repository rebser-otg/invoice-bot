package forwarder_test

import (
	"errors"
	"testing"

	"github.com/rebser-otg/invoice-bot/config"
	"github.com/rebser-otg/invoice-bot/forwarder"
	"github.com/rebser-otg/invoice-bot/memory"
)

type mockClient struct {
	searchIDs []string
	searchErr error
	rawByID   map[string][]byte
	fetchErr  error
	sentCount int
	sendErr   error
}

func (m *mockClient) Search(_ []string) ([]string, error) {
	return m.searchIDs, m.searchErr
}
func (m *mockClient) FetchRaw(id string) ([]byte, error) {
	if m.fetchErr != nil {
		return nil, m.fetchErr
	}
	return m.rawByID[id], nil
}
func (m *mockClient) Send(_ []byte) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sentCount++
	return nil
}

func testConfig() *config.Config {
	return &config.Config{
		ForwardTo:   "fwd@example.com",
		Senders:     []string{"billing@anthropic.com"},
		MessageText: "Please see this invoice.\n\n---\n",
	}
}

// rawMsg returns a minimal valid RFC 2822 message for the given ID.
func rawMsg(id string) []byte {
	return []byte(
		"From: billing@anthropic.com\r\n" +
			"Subject: Invoice " + id + "\r\n" +
			"Content-Type: text/plain\r\n" +
			"\r\nInvoice body.",
	)
}

func emptyMem(t *testing.T) *memory.Memory {
	t.Helper()
	m, err := memory.Load(t.TempDir() + "/memory.json")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestRun_ForwardsNewMessages(t *testing.T) {
	mem := emptyMem(t)
	client := &mockClient{
		searchIDs: []string{"id-1", "id-2"},
		rawByID:   map[string][]byte{"id-1": rawMsg("id-1"), "id-2": rawMsg("id-2")},
	}
	result, err := forwarder.Run(testConfig(), mem, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Forwarded != 2 {
		t.Errorf("Forwarded = %d, want 2", result.Forwarded)
	}
	if result.Failed != 0 {
		t.Errorf("Failed = %d, want 0", result.Failed)
	}
	if client.sentCount != 2 {
		t.Errorf("sent %d messages, want 2", client.sentCount)
	}
	if !mem.Contains("id-1") || !mem.Contains("id-2") {
		t.Error("forwarded IDs should be added to memory")
	}
}

func TestRun_SkipsAlreadySeen(t *testing.T) {
	mem := emptyMem(t)
	mem.Add("id-1")
	client := &mockClient{
		searchIDs: []string{"id-1", "id-2"},
		rawByID:   map[string][]byte{"id-2": rawMsg("id-2")},
	}
	result, err := forwarder.Run(testConfig(), mem, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Forwarded != 1 {
		t.Errorf("Forwarded = %d, want 1", result.Forwarded)
	}
	if result.AlreadySeen != 1 {
		t.Errorf("AlreadySeen = %d, want 1", result.AlreadySeen)
	}
}

func TestRun_SendFailure_NotAddedToMemory(t *testing.T) {
	mem := emptyMem(t)
	client := &mockClient{
		searchIDs: []string{"id-1"},
		rawByID:   map[string][]byte{"id-1": rawMsg("id-1")},
		sendErr:   errors.New("network error"),
	}
	result, err := forwarder.Run(testConfig(), mem, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Failed != 1 {
		t.Errorf("Failed = %d, want 1", result.Failed)
	}
	if mem.Contains("id-1") {
		t.Error("failed message ID must not be added to memory")
	}
}

func TestRun_NoMessages(t *testing.T) {
	mem := emptyMem(t)
	client := &mockClient{searchIDs: []string{}}
	result, err := forwarder.Run(testConfig(), mem, client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Forwarded != 0 || result.Failed != 0 || result.AlreadySeen != 0 {
		t.Errorf("expected all zeros, got %+v", result)
	}
}
