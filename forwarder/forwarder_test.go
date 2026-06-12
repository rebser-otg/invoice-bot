package forwarder_test

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/rebser-otg/invoice-bot/config"
	"github.com/rebser-otg/invoice-bot/forwarder"
	"github.com/rebser-otg/invoice-bot/gmail"
	"github.com/rebser-otg/invoice-bot/memory"
)

type mockClient struct {
	searchIDs []string
	searchErr error
	rawByID   map[string][]byte
	fetchErr  error
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

type mockUploader struct {
	uploaded []gmail.Attachment
	err      error
}

func (m *mockUploader) Upload(att gmail.Attachment) error {
	if m.err != nil {
		return m.err
	}
	m.uploaded = append(m.uploaded, att)
	return nil
}

func testConfig() *config.Config {
	return &config.Config{
		APIBaseURL: "https://hub.example.com",
		APIToken:   "otg_token",
		Senders:    []string{"billing@anthropic.com"},
	}
}

// rawWithPDF returns a multipart email carrying a base64 PDF attachment.
func rawWithPDF(id string) []byte {
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\n% mock invoice " + id + "\n"))
	return []byte(
		"From: billing@anthropic.com\r\n" +
			"Subject: Invoice " + id + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/mixed; boundary=BOUND\r\n\r\n" +
			"--BOUND\r\n" +
			"Content-Type: text/plain\r\n\r\nSee attached invoice.\r\n" +
			"--BOUND\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"Content-Disposition: attachment; filename=\"invoice-" + id + ".pdf\"\r\n\r\n" +
			pdf + "\r\n" +
			"--BOUND--\r\n",
	)
}

// rawWithInvoiceAndReceipt carries two PDFs: one named *invoice*, one not.
func rawWithInvoiceAndReceipt(id string) []byte {
	inv := base64.StdEncoding.EncodeToString([]byte("%PDF invoice " + id))
	rec := base64.StdEncoding.EncodeToString([]byte("%PDF receipt " + id))
	return []byte(
		"From: billing@anthropic.com\r\n" +
			"Subject: Invoice " + id + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/mixed; boundary=BOUND\r\n\r\n" +
			"--BOUND\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"Content-Disposition: attachment; filename=\"invoice-" + id + ".pdf\"\r\n\r\n" +
			inv + "\r\n" +
			"--BOUND\r\n" +
			"Content-Type: application/pdf\r\n" +
			"Content-Transfer-Encoding: base64\r\n" +
			"Content-Disposition: attachment; filename=\"receipt-" + id + ".pdf\"\r\n\r\n" +
			rec + "\r\n" +
			"--BOUND--\r\n",
	)
}

// rawPlain returns a plain-text email with NO attachment.
func rawPlain(id string) []byte {
	return []byte(
		"From: billing@anthropic.com\r\n" +
			"Subject: Invoice " + id + "\r\n" +
			"Content-Type: text/plain\r\n\r\nYour invoice is at https://example.com/inv.",
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

func TestRun_UploadsNewMessages(t *testing.T) {
	mem := emptyMem(t)
	client := &mockClient{
		searchIDs: []string{"id-1", "id-2"},
		rawByID:   map[string][]byte{"id-1": rawWithPDF("id-1"), "id-2": rawWithPDF("id-2")},
	}
	up := &mockUploader{}
	res, err := forwarder.Run(testConfig(), mem, client, up)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Uploaded != 2 {
		t.Errorf("Uploaded = %d, want 2", res.Uploaded)
	}
	if res.Failed != 0 || res.Skipped != 0 {
		t.Errorf("unexpected counts: %+v", res)
	}
	if len(up.uploaded) != 2 {
		t.Errorf("uploaded %d attachments, want 2", len(up.uploaded))
	}
	if up.uploaded[0].MimeType != "application/pdf" {
		t.Errorf("MimeType = %q, want application/pdf", up.uploaded[0].MimeType)
	}
	if !mem.Contains("id-1") || !mem.Contains("id-2") {
		t.Error("uploaded IDs should be added to memory")
	}
}

func TestRun_SkipsAlreadySeen(t *testing.T) {
	mem := emptyMem(t)
	mem.Add("id-1")
	client := &mockClient{
		searchIDs: []string{"id-1", "id-2"},
		rawByID:   map[string][]byte{"id-2": rawWithPDF("id-2")},
	}
	res, err := forwarder.Run(testConfig(), mem, client, &mockUploader{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Uploaded != 1 || res.AlreadySeen != 1 {
		t.Errorf("got %+v, want Uploaded=1 AlreadySeen=1", res)
	}
}

func TestRun_UploadFailure_NotAddedToMemory(t *testing.T) {
	mem := emptyMem(t)
	client := &mockClient{
		searchIDs: []string{"id-1"},
		rawByID:   map[string][]byte{"id-1": rawWithPDF("id-1")},
	}
	up := &mockUploader{err: errors.New("intake 500")}
	res, err := forwarder.Run(testConfig(), mem, client, up)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Failed != 1 {
		t.Errorf("Failed = %d, want 1", res.Failed)
	}
	if mem.Contains("id-1") {
		t.Error("failed message ID must not be added to memory (so it retries)")
	}
}

func TestRun_NoAttachment_SkippedAndMarkedSeen(t *testing.T) {
	mem := emptyMem(t)
	client := &mockClient{
		searchIDs: []string{"id-1"},
		rawByID:   map[string][]byte{"id-1": rawPlain("id-1")},
	}
	up := &mockUploader{}
	res, err := forwarder.Run(testConfig(), mem, client, up)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Skipped != 1 || res.Uploaded != 0 {
		t.Errorf("got %+v, want Skipped=1 Uploaded=0", res)
	}
	if len(up.uploaded) != 0 {
		t.Error("nothing should be uploaded for an attachment-less email")
	}
	if !mem.Contains("id-1") {
		t.Error("skipped email must be marked seen so it doesn't recur every run")
	}
}

func TestRun_MultiplePDFs_UploadsOnlyInvoice(t *testing.T) {
	mem := emptyMem(t)
	client := &mockClient{
		searchIDs: []string{"id-1"},
		rawByID:   map[string][]byte{"id-1": rawWithInvoiceAndReceipt("id-1")},
	}
	up := &mockUploader{}
	res, err := forwarder.Run(testConfig(), mem, client, up)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Uploaded != 1 {
		t.Errorf("Uploaded = %d, want 1", res.Uploaded)
	}
	if len(up.uploaded) != 1 {
		t.Fatalf("uploaded %d files, want 1 (only the invoice)", len(up.uploaded))
	}
	if up.uploaded[0].Filename != "invoice-id-1.pdf" {
		t.Errorf("uploaded %q, want the invoice PDF", up.uploaded[0].Filename)
	}
}

func TestRun_NoMessages(t *testing.T) {
	mem := emptyMem(t)
	res, err := forwarder.Run(testConfig(), mem, &mockClient{searchIDs: []string{}}, &mockUploader{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Uploaded != 0 || res.Failed != 0 || res.Skipped != 0 || res.AlreadySeen != 0 {
		t.Errorf("expected all zeros, got %+v", res)
	}
}
