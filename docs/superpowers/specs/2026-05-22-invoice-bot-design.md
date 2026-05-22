# Invoice Bot — Design Spec
_Date: 2026-05-22_

## Overview

A manually-triggered Go CLI tool that connects to a Google Workspace Gmail account, searches for invoice emails from Anthropic and OpenAI, and forwards any unseen ones to a configurable email address. A local JSON file tracks which messages have already been forwarded to prevent duplicates.

---

## Goals

- Connect to Gmail via the Gmail API (OAuth2 installed-app flow)
- Find invoice emails from a configurable list of senders
- Forward new invoices to a configurable destination address
- Never forward the same invoice twice (local memory)
- Run manually on demand (no daemon, no scheduler)

## Non-Goals

- Automatic scheduling / cron (can be added later)
- Multi-account support
- Attachment extraction or invoice parsing
- UI of any kind

---

## Tech Stack

- **Language:** Go
- **Gmail access:** Gmail API v1 via `google.golang.org/api/gmail/v1`
- **Auth:** OAuth2 installed-app flow via `golang.org/x/oauth2/google`
- **Config:** YAML via `gopkg.in/yaml.v3`

---

## Project Layout

```
invoice-bot/
├── main.go                         # entry point — wires components, prints summary
├── config/
│   └── config.go                   # loads config.yaml
├── gmail/
│   └── client.go                   # OAuth2 auth, Gmail search, fetch raw, send
├── memory/
│   └── memory.go                   # load/save memory.json, dedup check
├── forwarder/
│   └── forwarder.go                # orchestrates: load → search → dedupe → forward → save
├── docs/
│   └── superpowers/specs/
│       └── 2026-05-22-invoice-bot-design.md
├── senders.txt                     # one sender email per line — committed to git
├── message.txt                     # forward email body template — committed to git
├── config.yaml                     # gitignored — contains forward_to address
├── memory.json                     # gitignored — auto-created, persists forwarded IDs
├── credentials.json                # gitignored — OAuth client secret from Google Cloud
├── token.json                      # gitignored — OAuth token, auto-created on first run
├── go.mod
├── go.sum
└── .gitignore
```

---

## Configuration Files

### config.yaml (gitignored)
```yaml
forward_to: accounting@example.com
```

### senders.txt (committed)
One sender email address per line. Blank lines and lines starting with `#` are ignored.
```
billing@anthropic.com
invoices@openai.com
```

### message.txt (committed)
Plain text body prepended to each forwarded email.
```
Please find below a forwarded invoice.

---
```

### memory.json (gitignored, auto-created)
```json
{
  "forwarded": ["<gmail-message-id-1>", "<gmail-message-id-2>"]
}
```

---

## Component Design

### `config/config.go`
- Loads `config.yaml` → `Config{ ForwardTo string }`
- Loads `senders.txt` → `[]string` (trims whitespace, skips blank lines and `#` comments)
- Loads `message.txt` → `string`
- Returns a clear error if any required file is missing or unparseable

### `memory/memory.go`
- `Load(path) → Memory` — reads `memory.json`; returns empty Memory if file doesn't exist
- `Memory.Contains(id string) bool`
- `Memory.Add(id string)`
- `Memory.Save(path) → error` — writes back atomically (write to temp file, rename)

### `gmail/client.go`
- `NewClient(credentialsPath, tokenPath) → Client` — loads OAuth2 credentials; if `token.json` is missing or expired, opens browser for consent flow and saves new token
- `Client.Search(senders []string) → []string` — builds query `from:(a@b.com OR c@d.com)`, returns message IDs
- `Client.FetchRaw(id string) → []byte` — fetches the raw RFC 2822 message bytes
- `Client.Send(raw []byte) → error` — sends a message via Gmail API

### `forwarder/forwarder.go`
- `Run(config, memory, gmailClient)` — the main loop:
  1. Search for messages matching senders
  2. Filter out IDs already in memory
  3. For each new ID: fetch raw → build forward email → send → add to memory
  4. Save memory
  5. Return `(forwarded int, failed int)`

### `main.go`
- Loads config, memory, constructs gmail client, calls `forwarder.Run`
- Prints: `Forwarded 3 new invoice(s). 2 failed. 12 already seen.`
- Exits non-zero if any step fails fatally

---

## Email Forwarding

The Gmail API has no native "forward" operation. The approach:

1. Fetch the raw RFC 2822 message bytes (`format=raw`)
2. Decode the base64url payload
3. Parse the MIME message using Go's `net/mail`
4. Build a new MIME message:
   - `To`: `config.forward_to`
   - `Subject`: `Fwd: <original subject>`
   - `Body`: contents of `message.txt` + `\n\n` + original plain-text body
   - Preserve original attachments
5. Base64url-encode and send via `gmail.users.messages.send`

---

## Auth Setup (first run)

1. User creates a Google Cloud project and enables the Gmail API
2. Downloads `credentials.json` (OAuth 2.0 Desktop App client) into the project root
3. Runs `./invoice-bot` — browser opens for consent, `token.json` is saved
4. Subsequent runs use the saved token (auto-refreshed)

A `README.md` will document this setup process step by step.

**Required OAuth scopes:**
- `https://www.googleapis.com/auth/gmail.readonly` — search and fetch messages
- `https://www.googleapis.com/auth/gmail.send` — send forwarded emails

---

## Error Handling

| Situation | Behaviour |
|---|---|
| `credentials.json` missing | Exit with message pointing to Google Cloud Console setup |
| Gmail API search/fetch error | Abort run, print error |
| Forward send fails for one message | Log failure, continue with remaining messages, do NOT add to memory (retry next run) |
| `config.yaml` missing or invalid | Exit with clear parse error |
| `senders.txt` missing | Exit with error |
| `message.txt` missing | Exit with error |
| `memory.json` missing | Auto-create empty memory, continue |
| No new invoices found | Print `No new invoices found.` and exit 0 |

---

## .gitignore

```
config.yaml
memory.json
credentials.json
token.json
invoice-bot          # compiled binary
```
