package uploader_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rebser-otg/invoice-bot/gmail"
	"github.com/rebser-otg/invoice-bot/uploader"
)

func sampleAttachment() gmail.Attachment {
	return gmail.Attachment{
		Filename: "rechnung.pdf",
		MimeType: "application/pdf",
		Data:     []byte("%PDF-1.4\nhello"),
	}
}

func TestUpload_PostsMultipartWithBearer(t *testing.T) {
	var gotAuth, gotPath, gotCT, gotFilename, gotPartCT string
	var gotFileBytes []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		f, hdr, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		defer f.Close()
		gotFilename = hdr.Filename
		gotPartCT = hdr.Header.Get("Content-Type")
		gotFileBytes, _ = io.ReadAll(f)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	u := uploader.New(srv.URL, "otg_secret")
	if err := u.Upload(sampleAttachment()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotAuth != "Bearer otg_secret" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotPath != "/api/expenses/intake" {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.HasPrefix(gotCT, "multipart/form-data") {
		t.Errorf("Content-Type = %q", gotCT)
	}
	if gotFilename != "rechnung.pdf" {
		t.Errorf("filename = %q", gotFilename)
	}
	if gotPartCT != "application/pdf" {
		t.Errorf("part Content-Type = %q, want application/pdf", gotPartCT)
	}
	if string(gotFileBytes) != "%PDF-1.4\nhello" {
		t.Errorf("file bytes = %q", gotFileBytes)
	}
}

func TestUpload_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	u := uploader.New(srv.URL, "bad")
	err := u.Upload(sampleAttachment())
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error should carry status + body, got: %v", err)
	}
}
