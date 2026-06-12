package gmail_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/rebser-otg/invoice-bot/gmail"
)

func multipartWith(parts string) []byte {
	return []byte(
		"From: billing@anthropic.com\r\n" +
			"Subject: Invoice\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: multipart/mixed; boundary=BOUND\r\n\r\n" +
			parts +
			"--BOUND--\r\n",
	)
}

func b64Part(contentType, filename, payload string) string {
	enc := base64.StdEncoding.EncodeToString([]byte(payload))
	return "--BOUND\r\n" +
		"Content-Type: " + contentType + "\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"Content-Disposition: attachment; filename=\"" + filename + "\"\r\n\r\n" +
		enc + "\r\n"
}

func TestExtract_PDFAttachment(t *testing.T) {
	payload := "%PDF-1.4\nhello"
	raw := multipartWith(
		"--BOUND\r\nContent-Type: text/plain\r\n\r\nbody text\r\n" +
			b64Part("application/pdf", "rechnung.pdf", payload),
	)
	atts, err := gmail.ExtractInvoiceAttachments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 1 {
		t.Fatalf("got %d attachments, want 1", len(atts))
	}
	if atts[0].Filename != "rechnung.pdf" {
		t.Errorf("Filename = %q", atts[0].Filename)
	}
	if atts[0].MimeType != "application/pdf" {
		t.Errorf("MimeType = %q", atts[0].MimeType)
	}
	if !bytes.Equal(atts[0].Data, []byte(payload)) {
		t.Errorf("base64 not decoded correctly: %q", atts[0].Data)
	}
}

func TestExtract_OctetStreamFallsBackToExtension(t *testing.T) {
	// Some mailers send PDFs as application/octet-stream — the .pdf filename
	// must still classify it.
	raw := multipartWith(b64Part("application/octet-stream", "beleg.pdf", "%PDF-1.4 x"))
	atts, err := gmail.ExtractInvoiceAttachments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 1 || atts[0].MimeType != "application/pdf" {
		t.Fatalf("got %+v, want one application/pdf", atts)
	}
}

func TestExtract_IgnoresNonInvoiceParts(t *testing.T) {
	raw := multipartWith(
		"--BOUND\r\nContent-Type: text/plain\r\n\r\njust text\r\n" +
			"--BOUND\r\nContent-Type: text/html\r\n\r\n<p>html</p>\r\n" +
			b64Part("application/zip", "stuff.zip", "PK\x03\x04junk"),
	)
	atts, err := gmail.ExtractInvoiceAttachments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 0 {
		t.Errorf("got %d attachments, want 0", len(atts))
	}
}

func TestExtract_MultipleAttachments(t *testing.T) {
	raw := multipartWith(
		b64Part("application/pdf", "a.pdf", "%PDF a") +
			b64Part("image/jpeg", "b.jpg", "\xff\xd8\xff jpeg"),
	)
	atts, err := gmail.ExtractInvoiceAttachments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 2 {
		t.Fatalf("got %d, want 2", len(atts))
	}
	if atts[1].MimeType != "image/jpeg" {
		t.Errorf("second MimeType = %q, want image/jpeg", atts[1].MimeType)
	}
}

func TestExtract_NestedMultipart(t *testing.T) {
	// multipart/mixed → multipart/alternative (text+html) + a PDF sibling.
	inner := "--ALT\r\nContent-Type: text/plain\r\n\r\nplain\r\n" +
		"--ALT\r\nContent-Type: text/html\r\n\r\n<p>html</p>\r\n--ALT--\r\n"
	raw := multipartWith(
		"--BOUND\r\nContent-Type: multipart/alternative; boundary=ALT\r\n\r\n" + inner +
			b64Part("application/pdf", "nested.pdf", "%PDF nested"),
	)
	atts, err := gmail.ExtractInvoiceAttachments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 1 || atts[0].Filename != "nested.pdf" {
		t.Fatalf("got %+v, want one nested.pdf", atts)
	}
}

func names(atts []gmail.Attachment) []string {
	out := make([]string, len(atts))
	for i, a := range atts {
		out[i] = a.Filename
	}
	return out
}

func TestSelectInvoices(t *testing.T) {
	att := func(name string) gmail.Attachment { return gmail.Attachment{Filename: name} }
	cases := []struct {
		name string
		in   []gmail.Attachment
		want []string
	}{
		{"single non-invoice kept", []gmail.Attachment{att("beleg.pdf")}, []string{"beleg.pdf"}},
		{"empty", nil, nil},
		{
			"multiple → only invoice-named",
			[]gmail.Attachment{att("Invoice-123.pdf"), att("receipt.pdf")},
			[]string{"Invoice-123.pdf"},
		},
		{
			"case-insensitive match",
			[]gmail.Attachment{att("MY_INVOICE.pdf"), att("quittung.pdf")},
			[]string{"MY_INVOICE.pdf"},
		},
		{
			"multiple invoices both kept",
			[]gmail.Attachment{att("invoice-a.pdf"), att("invoice-b.pdf"), att("receipt.pdf")},
			[]string{"invoice-a.pdf", "invoice-b.pdf"},
		},
		{
			"multiple, none named invoice → all kept",
			[]gmail.Attachment{att("doc1.pdf"), att("doc2.pdf")},
			[]string{"doc1.pdf", "doc2.pdf"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := names(gmail.SelectInvoices(tc.in))
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got %v, want %v", got, tc.want)
					break
				}
			}
		})
	}
}

func TestExtract_NonMultipartPlainText(t *testing.T) {
	raw := []byte("From: x@y.com\r\nContent-Type: text/plain\r\n\r\njust a body, no attachment")
	atts, err := gmail.ExtractInvoiceAttachments(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(atts) != 0 {
		t.Errorf("got %d, want 0", len(atts))
	}
}
