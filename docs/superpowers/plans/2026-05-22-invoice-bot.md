# Invoice Bot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI that connects to Gmail via OAuth2, finds invoice emails from configurable senders, and forwards unseen ones to a configurable address — tracking forwarded IDs in a local JSON file.

**Architecture:** Four packages (`config`, `memory`, `gmail`, `forwarder`) wired in `main.go`. `gmail.Client` is accessed through a `MailClient` interface so the forwarder is fully testable without real API calls. `gmail.BuildForward` is a pure function (no API calls) that constructs the forwarded MIME message.

**Tech Stack:** Go 1.22+, `golang.org/x/oauth2/google`, `google.golang.org/api/gmail/v1`, `gopkg.in/yaml.v3`

---

## File Map

| File | Role |
|---|---|
| `main.go` | Entry point — wires components, prints summary |
| `config/config.go` | Loads `config.yaml`, `senders.txt`, `message.txt` |
| `config/config_test.go` | Unit tests for config loading |
| `memory/memory.go` | Load/save `memory.json`, dedup by message ID |
| `memory/memory_test.go` | Unit tests for memory load/save/contains |
| `gmail/client.go` | OAuth2 auth, Gmail search, fetch raw message, send |
| `gmail/client_test.go` | Unit tests for `BuildQuery`, `NewClient` errors, interface check |
| `gmail/mime.go` | Pure function: builds forwarded MIME message from raw bytes |
| `gmail/mime_test.go` | Unit tests for `BuildForward` |
| `forwarder/forwarder.go` | Orchestrates search → dedupe → forward → save |
| `forwarder/forwarder_test.go` | Unit tests using mock `MailClient` |
| `senders.txt` | One sender per line, committed |
| `message.txt` | Forward email body template, committed |
| `config.yaml.example` | Example config, committed |
| `.gitignore` | Excludes secrets and binary |
| `README.md` | Setup instructions |

---

### Task 1: Initialize module and scaffolding

**Files:**
- Create: `go.mod`
- Create: `.gitignore`
- Create: `senders.txt`
- Create: `message.txt`
- Create: `config.yaml.example`

- [ ] **Step 1: Initialize Go module**
```bash
cd /home/robin/Documents/OTG/invoice-bot
go mod init github.com/rebser-otg/invoice-bot
```
Expected: `go.mod` created

- [ ] **Step 2: Create `.gitignore`**
```
config.yaml
memory.json
credentials.json
token.json
invoice-bot
```

- [ ] **Step 3: Create `senders.txt`**
```
# One sender email per line. Lines starting with # are ignored.
billing@anthropic.com
invoices@openai.com
```

- [ ] **Step 4: Create `message.txt`**
```
Please find below a forwarded invoice.

---
```

- [ ] **Step 5: Create `config.yaml.example`**
```yaml
forward_to: accounting@example.com
```

- [ ] **Step 6: Commit**
```bash
git add go.mod .gitignore senders.txt message.txt config.yaml.example
git commit -m "chore: initialize Go module and project scaffolding"
```

---

### Task 2: config package

**Files:**
- Create: `config/config.go`
- Create: `config/config_test.go`

- [ ] **Step 1: Install yaml dependency**
```bash
go get gopkg.in/yaml.v3
```

- [ ] **Step 2: Write failing tests**

Create `config/config_test.go`:
```go
package config_test

import (
	"os"
	"path/filepath"
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
```

- [ ] **Step 3: Run tests to verify they fail**
```bash
go test ./config/...
```
Expected: compile error — `config` package does not exist

- [ ] **Step 4: Implement `config/config.go`**
```go
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all loaded configuration.
type Config struct {
	ForwardTo   string
	Senders     []string
	MessageText string
}

type yamlConfig struct {
	ForwardTo string `yaml:"forward_to"`
}

// Load reads config.yaml, senders.txt, and message.txt from dir.
func Load(dir string) (*Config, error) {
	yc, err := loadYAML(filepath.Join(dir, "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("config.yaml: %w", err)
	}
	senders, err := loadSenders(filepath.Join(dir, "senders.txt"))
	if err != nil {
		return nil, fmt.Errorf("senders.txt: %w", err)
	}
	msg, err := os.ReadFile(filepath.Join(dir, "message.txt"))
	if err != nil {
		return nil, fmt.Errorf("message.txt: %w", err)
	}
	return &Config{
		ForwardTo:   yc.ForwardTo,
		Senders:     senders,
		MessageText: string(msg),
	}, nil
}

func loadYAML(path string) (*yamlConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var yc yamlConfig
	if err := yaml.Unmarshal(data, &yc); err != nil {
		return nil, err
	}
	return &yc, nil
}

func loadSenders(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var senders []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		senders = append(senders, line)
	}
	return senders, scanner.Err()
}
```

- [ ] **Step 5: Run tests to verify they pass**
```bash
go test ./config/... -v
```
Expected: all 4 tests PASS

- [ ] **Step 6: Commit**
```bash
git add config/ go.mod go.sum
git commit -m "feat: add config package"
```

---

### Task 3: memory package

**Files:**
- Create: `memory/memory.go`
- Create: `memory/memory_test.go`

- [ ] **Step 1: Write failing tests**

Create `memory/memory_test.go`:
```go
package memory_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rebser-otg/invoice-bot/memory"
)

func TestLoad_EmptyWhenMissing(t *testing.T) {
	dir := t.TempDir()
	m, err := memory.Load(filepath.Join(dir, "memory.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Contains("any-id") {
		t.Error("expected empty memory to not contain any ID")
	}
}

func TestMemory_AddAndContains(t *testing.T) {
	dir := t.TempDir()
	m, _ := memory.Load(filepath.Join(dir, "memory.json"))
	m.Add("msg-1")
	m.Add("msg-2")

	if !m.Contains("msg-1") {
		t.Error("expected msg-1 to be present")
	}
	if !m.Contains("msg-2") {
		t.Error("expected msg-2 to be present")
	}
	if m.Contains("msg-3") {
		t.Error("expected msg-3 to be absent")
	}
}

func TestMemory_SaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")

	m, _ := memory.Load(path)
	m.Add("msg-abc")
	if err := m.Save(path); err != nil {
		t.Fatalf("save error: %v", err)
	}

	m2, err := memory.Load(path)
	if err != nil {
		t.Fatalf("reload error: %v", err)
	}
	if !m2.Contains("msg-abc") {
		t.Error("expected msg-abc to persist after save/reload")
	}
}

func TestMemory_AtomicSave_NoTempFileLeft(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	m, _ := memory.Load(path)
	m.Add("msg-1")
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected 1 file after atomic save, got %d: %v", len(entries), entries)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**
```bash
go test ./memory/...
```
Expected: compile error — `memory` package does not exist

- [ ] **Step 3: Implement `memory/memory.go`**
```go
package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// Memory tracks which Gmail message IDs have been forwarded.
type Memory struct {
	Forwarded []string `json:"forwarded"`
	index     map[string]struct{}
}

// Load reads memory.json from path. Returns an empty Memory if the file does not exist.
func Load(path string) (*Memory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Memory{index: make(map[string]struct{})}, nil
		}
		return nil, fmt.Errorf("reading memory: %w", err)
	}
	var m Memory
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing memory: %w", err)
	}
	m.index = make(map[string]struct{}, len(m.Forwarded))
	for _, id := range m.Forwarded {
		m.index[id] = struct{}{}
	}
	return &m, nil
}

// Contains reports whether id has been forwarded.
func (m *Memory) Contains(id string) bool {
	_, ok := m.index[id]
	return ok
}

// Add records id as forwarded. No-op if already present.
func (m *Memory) Add(id string) {
	if !m.Contains(id) {
		m.Forwarded = append(m.Forwarded, id)
		m.index[id] = struct{}{}
	}
}

// Len returns the number of forwarded IDs.
func (m *Memory) Len() int {
	return len(m.Forwarded)
}

// Save writes memory to path atomically (write to temp, then rename).
func (m *Memory) Save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling memory: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**
```bash
go test ./memory/... -v
```
Expected: all 4 tests PASS

- [ ] **Step 5: Commit**
```bash
git add memory/
git commit -m "feat: add memory package"
```

---

### Task 4: Gmail client — auth

**Files:**
- Create: `gmail/client.go`
- Create: `gmail/client_test.go`

- [ ] **Step 1: Install Gmail API dependencies**
```bash
go get google.golang.org/api/gmail/v1
go get golang.org/x/oauth2/google
```

- [ ] **Step 2: Write failing test**

Create `gmail/client_test.go`:
```go
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
```

- [ ] **Step 3: Run test to verify it fails**
```bash
go test ./gmail/... -run TestNewClient_MissingCredentials
```
Expected: compile error — `gmail` package does not exist

- [ ] **Step 4: Implement `gmail/client.go`**
```go
package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gmailv1 "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

var scopes = []string{
	gmailv1.GmailReadonlyScope,
	gmailv1.GmailSendScope,
}

// Client wraps the Gmail API service.
type Client struct {
	svc *gmailv1.Service
}

// NewClient loads OAuth2 credentials and returns an authenticated Client.
// If token.json is missing or expired, it prints a URL for the consent flow,
// reads the auth code from stdin, and saves the resulting token to tokenPath.
func NewClient(credentialsPath, tokenPath string) (*Client, error) {
	data, err := os.ReadFile(credentialsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf(
				"credentials.json not found at %q\n"+
					"Download it from https://console.cloud.google.com/apis/credentials\n"+
					"(Create OAuth 2.0 Client ID → Desktop App, then download JSON)",
				credentialsPath,
			)
		}
		return nil, fmt.Errorf("reading credentials: %w", err)
	}

	oauthCfg, err := google.ConfigFromJSON(data, scopes...)
	if err != nil {
		return nil, fmt.Errorf("parsing credentials: %w", err)
	}

	tok, err := loadToken(tokenPath)
	if err != nil {
		tok, err = runBrowserFlow(oauthCfg)
		if err != nil {
			return nil, fmt.Errorf("OAuth flow: %w", err)
		}
		if err := saveToken(tokenPath, tok); err != nil {
			return nil, fmt.Errorf("saving token: %w", err)
		}
	}

	httpClient := oauthCfg.Client(context.Background(), tok)
	svc, err := gmailv1.NewService(context.Background(), option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("creating Gmail service: %w", err)
	}
	return &Client{svc: svc}, nil
}

func loadToken(path string) (*oauth2.Token, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tok oauth2.Token
	if err := json.Unmarshal(data, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveToken(path string, tok *oauth2.Token) error {
	data, err := json.Marshal(tok)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func runBrowserFlow(cfg *oauth2.Config) (*oauth2.Token, error) {
	cfg.RedirectURL = "urn:ietf:wg:oauth:2.0:oob"
	authURL := cfg.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Open this URL in your browser to authorize invoice-bot:\n\n%s\n\nEnter the authorization code: ", authURL)
	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("reading auth code: %w", err)
	}
	tok, err := cfg.Exchange(context.Background(), code)
	if err != nil {
		return nil, fmt.Errorf("exchanging code: %w", err)
	}
	return tok, nil
}
```

- [ ] **Step 5: Run test to verify it passes**
```bash
go test ./gmail/... -run TestNewClient_MissingCredentials -v
```
Expected: PASS

- [ ] **Step 6: Commit**
```bash
git add gmail/client.go gmail/client_test.go go.mod go.sum
git commit -m "feat: add gmail client with OAuth2 auth"
```

---

### Task 5: Gmail client — search and fetch

**Files:**
- Modify: `gmail/client.go`
- Modify: `gmail/client_test.go`

- [ ] **Step 1: Write failing tests for BuildQuery**

Append to `gmail/client_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they fail**
```bash
go test ./gmail/... -run TestBuildQuery
```
Expected: FAIL — `BuildQuery` undefined

- [ ] **Step 3: Add `BuildQuery`, `Search`, and `FetchRaw` to `gmail/client.go`**

Add to the import block in `gmail/client.go`:
```go
"encoding/base64"
"strings"
```

Append to `gmail/client.go`:
```go
// BuildQuery constructs a Gmail search query matching any of the given sender addresses.
func BuildQuery(senders []string) string {
	return "from:(" + strings.Join(senders, " OR ") + ")"
}

// Search returns all Gmail message IDs matching the configured senders.
// Handles pagination automatically.
func (c *Client) Search(senders []string) ([]string, error) {
	query := BuildQuery(senders)
	var ids []string
	pageToken := ""
	for {
		call := c.svc.Users.Messages.List("me").Q(query)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		r, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("gmail search: %w", err)
		}
		for _, m := range r.Messages {
			ids = append(ids, m.Id)
		}
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return ids, nil
}

// FetchRaw retrieves the raw RFC 2822 bytes for the given Gmail message ID.
func (c *Client) FetchRaw(id string) ([]byte, error) {
	msg, err := c.svc.Users.Messages.Get("me", id).Format("raw").Do()
	if err != nil {
		return nil, fmt.Errorf("fetching message %s: %w", id, err)
	}
	// Gmail uses base64url; try with padding first, then without.
	raw, err := base64.URLEncoding.DecodeString(msg.Raw)
	if err != nil {
		raw, err = base64.RawURLEncoding.DecodeString(msg.Raw)
		if err != nil {
			return nil, fmt.Errorf("decoding message %s: %w", id, err)
		}
	}
	return raw, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**
```bash
go test ./gmail/... -v
```
Expected: `TestBuildQuery_Multiple`, `TestBuildQuery_Single`, `TestNewClient_MissingCredentials` all PASS

- [ ] **Step 5: Commit**
```bash
git add gmail/
git commit -m "feat: add gmail search and fetch"
```

---

### Task 6: Build forward MIME message

**Files:**
- Create: `gmail/mime.go`
- Create: `gmail/mime_test.go`

- [ ] **Step 1: Write failing tests**

Create `gmail/mime_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they fail**
```bash
go test ./gmail/... -run TestBuildForward
```
Expected: FAIL — `BuildForward` undefined

- [ ] **Step 3: Implement `gmail/mime.go`**
```go
package gmail

import (
	"bytes"
	"fmt"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/textproto"
	"strings"
)

// BuildForward constructs a forwarded email as RFC 2822 bytes.
// The result is a multipart/mixed message:
//   - Part 1: text/plain with messageText (from message.txt)
//   - Part 2: message/rfc822 with the original raw email
func BuildForward(rawOriginal []byte, forwardTo, messageText string) ([]byte, error) {
	origMsg, err := mail.ReadMessage(bytes.NewReader(rawOriginal))
	if err != nil {
		return nil, fmt.Errorf("parsing original message: %w", err)
	}

	subject := origMsg.Header.Get("Subject")
	dec := new(mime.WordDecoder)
	if decoded, err := dec.DecodeHeader(subject); err == nil {
		subject = decoded
	}
	if !strings.HasPrefix(subject, "Fwd: ") {
		subject = "Fwd: " + subject
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// Write outer headers before any multipart boundary
	fmt.Fprintf(&buf, "To: %s\r\n", forwardTo)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", w.Boundary())

	// Part 1: the message.txt preamble
	hdr1 := textproto.MIMEHeader{}
	hdr1.Set("Content-Type", "text/plain; charset=utf-8")
	p1, err := w.CreatePart(hdr1)
	if err != nil {
		return nil, fmt.Errorf("creating text part: %w", err)
	}
	fmt.Fprint(p1, messageText)

	// Part 2: the original email as an RFC 2822 attachment
	hdr2 := textproto.MIMEHeader{}
	hdr2.Set("Content-Type", "message/rfc822")
	hdr2.Set("Content-Disposition", `attachment; filename="original.eml"`)
	p2, err := w.CreatePart(hdr2)
	if err != nil {
		return nil, fmt.Errorf("creating rfc822 part: %w", err)
	}
	if _, err := p2.Write(rawOriginal); err != nil {
		return nil, fmt.Errorf("writing original message: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}
	return buf.Bytes(), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**
```bash
go test ./gmail/... -v
```
Expected: all tests in `gmail` package PASS

- [ ] **Step 5: Commit**
```bash
git add gmail/mime.go gmail/mime_test.go
git commit -m "feat: add MIME forward email builder"
```

---

### Task 7: Gmail client — send

**Files:**
- Modify: `gmail/client.go`
- Modify: `gmail/client_test.go`

- [ ] **Step 1: Write interface compliance test**

Append to `gmail/client_test.go`:
```go
// TestClient_ImplementsMailClient verifies Client satisfies the forwarder.MailClient interface at compile time.
func TestClient_ImplementsMailClient(t *testing.T) {
	var _ interface {
		Search([]string) ([]string, error)
		FetchRaw(string) ([]byte, error)
		Send([]byte) error
	} = (*gmail.Client)(nil)
}
```

- [ ] **Step 2: Run test to verify it fails**
```bash
go test ./gmail/... -run TestClient_ImplementsMailClient
```
Expected: FAIL — `*gmail.Client` does not implement `Send`

- [ ] **Step 3: Add `Send` to `gmail/client.go`**

Append to `gmail/client.go`:
```go
// Send encodes raw RFC 2822 bytes as base64url and sends them via the Gmail API.
func (c *Client) Send(raw []byte) error {
	encoded := base64.URLEncoding.EncodeToString(raw)
	msg := &gmailv1.Message{Raw: encoded}
	_, err := c.svc.Users.Messages.Send("me", msg).Do()
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**
```bash
go test ./gmail/... -v
```
Expected: all tests PASS including `TestClient_ImplementsMailClient`

- [ ] **Step 5: Commit**
```bash
git add gmail/
git commit -m "feat: add gmail send"
```

---

### Task 8: forwarder package

**Files:**
- Create: `forwarder/forwarder.go`
- Create: `forwarder/forwarder_test.go`

- [ ] **Step 1: Write failing tests**

Create `forwarder/forwarder_test.go`:
```go
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
```

- [ ] **Step 2: Run tests to verify they fail**
```bash
go test ./forwarder/...
```
Expected: compile error — `forwarder` package does not exist

- [ ] **Step 3: Implement `forwarder/forwarder.go`**
```go
package forwarder

import (
	"fmt"

	"github.com/rebser-otg/invoice-bot/config"
	"github.com/rebser-otg/invoice-bot/gmail"
	"github.com/rebser-otg/invoice-bot/memory"
)

// MailClient is the interface the forwarder uses to interact with Gmail.
// *gmail.Client satisfies this interface.
type MailClient interface {
	Search(senders []string) ([]string, error)
	FetchRaw(id string) ([]byte, error)
	Send(raw []byte) error
}

// Result holds the outcome of a Run call.
type Result struct {
	Forwarded   int
	Failed      int
	AlreadySeen int
}

// Run searches for invoice emails, filters out already-forwarded ones,
// forwards new ones via client, and updates mem.
// mem is NOT saved to disk — the caller is responsible for calling mem.Save.
func Run(cfg *config.Config, mem *memory.Memory, client MailClient) (Result, error) {
	ids, err := client.Search(cfg.Senders)
	if err != nil {
		return Result{}, fmt.Errorf("searching gmail: %w", err)
	}

	var res Result
	for _, id := range ids {
		if mem.Contains(id) {
			res.AlreadySeen++
			continue
		}

		raw, err := client.FetchRaw(id)
		if err != nil {
			fmt.Printf("  [error] fetching %s: %v\n", id, err)
			res.Failed++
			continue
		}

		fwd, err := gmail.BuildForward(raw, cfg.ForwardTo, cfg.MessageText)
		if err != nil {
			fmt.Printf("  [error] building forward for %s: %v\n", id, err)
			res.Failed++
			continue
		}

		if err := client.Send(fwd); err != nil {
			fmt.Printf("  [error] sending %s: %v\n", id, err)
			res.Failed++
			continue
		}

		mem.Add(id)
		res.Forwarded++
	}

	return res, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**
```bash
go test ./forwarder/... -v
```
Expected: all 4 tests PASS

- [ ] **Step 5: Run all tests**
```bash
go test ./...
```
Expected: all tests across all packages PASS

- [ ] **Step 6: Commit**
```bash
git add forwarder/
git commit -m "feat: add forwarder package"
```

---

### Task 9: main.go

**Files:**
- Create: `main.go`

- [ ] **Step 1: Write `main.go`**
```go
package main

import (
	"fmt"
	"os"

	"github.com/rebser-otg/invoice-bot/config"
	"github.com/rebser-otg/invoice-bot/forwarder"
	"github.com/rebser-otg/invoice-bot/gmail"
	"github.com/rebser-otg/invoice-bot/memory"
)

func main() {
	// All files are expected relative to the current working directory.
	// Run this binary from the project root.
	cfg, err := config.Load(".")
	if err != nil {
		fatalf("loading config: %v", err)
	}

	const memPath = "memory.json"
	mem, err := memory.Load(memPath)
	if err != nil {
		fatalf("loading memory: %v", err)
	}

	client, err := gmail.NewClient("credentials.json", "token.json")
	if err != nil {
		fatalf("connecting to Gmail:\n%v", err)
	}

	result, err := forwarder.Run(cfg, mem, client)
	if err != nil {
		fatalf("running forwarder: %v", err)
	}

	if err := mem.Save(memPath); err != nil {
		fatalf("saving memory: %v", err)
	}

	if result.Forwarded == 0 && result.Failed == 0 {
		fmt.Println("No new invoices found.")
		return
	}
	fmt.Printf("Forwarded %d new invoice(s). %d failed. %d already seen.\n",
		result.Forwarded, result.Failed, result.AlreadySeen)
	if result.Failed > 0 {
		os.Exit(1)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
```

- [ ] **Step 2: Verify full build**
```bash
go build -o invoice-bot .
```
Expected: binary `invoice-bot` created, no errors

- [ ] **Step 3: Run all tests one final time**
```bash
go test ./...
```
Expected: all tests PASS

- [ ] **Step 4: Commit**
```bash
git add main.go
git commit -m "feat: add main entrypoint"
```

---

### Task 10: README.md

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write `README.md`**
```markdown
# invoice-bot

A CLI tool that scans your Gmail for invoices from Anthropic and OpenAI and forwards new ones to a configured email address. Keeps a local record of already-forwarded messages to avoid duplicates.

## Setup

### 1. Google Cloud project and credentials

1. Go to [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or select an existing one)
3. Enable the **Gmail API**: APIs & Services → Enable APIs → search "Gmail API" → Enable
4. Create credentials: APIs & Services → Credentials → **Create Credentials** → **OAuth 2.0 Client ID**
   - Application type: **Desktop App**
   - Name: `invoice-bot` (or anything)
   - Click Create, then **Download JSON**
5. Save the downloaded file as `credentials.json` in the project root

### 2. Configure

```bash
cp config.yaml.example config.yaml
```

Edit `config.yaml`:
```yaml
forward_to: your-accounting-address@example.com
```

Edit `senders.txt` to add or remove invoice sender addresses:
```
billing@anthropic.com
invoices@openai.com
```

Edit `message.txt` to customize the text that appears at the top of each forwarded email.

### 3. Build

```bash
go build -o invoice-bot .
```

### 4. First run — OAuth consent

```bash
./invoice-bot
```

On first run a URL is printed. Open it in your browser, sign in with your Google Workspace account, grant access, and paste the authorization code back into the terminal. The token is saved to `token.json` — subsequent runs are fully automatic.

### 5. Subsequent runs

```bash
./invoice-bot
```

Example output:
```
Forwarded 3 new invoice(s). 0 failed. 12 already seen.
```

Or if nothing new:
```
No new invoices found.
```

Exit code is `1` if any forwards failed (so they will be retried on the next run).

## Files

| File | Purpose | Committed |
|---|---|---|
| `config.yaml` | Forwarding address | ❌ gitignored |
| `senders.txt` | Invoice sender addresses | ✅ |
| `message.txt` | Forward email body template | ✅ |
| `credentials.json` | Google OAuth client secret | ❌ gitignored |
| `token.json` | Saved OAuth token (auto-created on first run) | ❌ gitignored |
| `memory.json` | Forwarded message IDs (auto-created) | ❌ gitignored |

## Notes

- Run `./invoice-bot` from the project root — all config files are resolved relative to the working directory.
- To reset forwarded history, delete `memory.json` (all matching invoices will be forwarded again).
- To re-authenticate, delete `token.json` and run again.
```

- [ ] **Step 2: Commit and push**
```bash
git add README.md
git commit -m "docs: add README with setup instructions"
git push
```
Expected: all commits pushed to `rebser-otg/invoice-bot`
