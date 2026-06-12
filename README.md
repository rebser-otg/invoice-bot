# invoice-bot

A CLI tool that scans your Gmail for invoices from Anthropic and OpenAI, extracts their PDF/image attachments, and uploads each to the OTG Office Hub expense intake (`POST /api/expenses/intake`) — creating a draft expense per invoice that you review and submit in the portal. Keeps a local record of already-processed messages to avoid duplicates.

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
api_base_url: https://offices.onetech.group
api_token: otg_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Generate the API token in Office Hub under **Profil → Sicherheit → API-Token** (shown once — copy it straight into `config.yaml`, which is gitignored).

Edit `senders.txt` to add or remove invoice sender addresses:
```
billing@anthropic.com
invoices@openai.com
```

### 3. Build

```bash
go build -o invoice-bot .
```

### 4. First run — OAuth consent

```bash
./invoice-bot
```

On first run the tool opens your browser automatically (or prints the URL if it can't). Sign in with your Google Workspace account and grant access — the browser will show "Authorization successful" and the token is saved to `token.json`. Subsequent runs are fully automatic.

### 5. Subsequent runs

```bash
./invoice-bot
```

Example output:
```
Uploaded 3 new invoice(s). 1 skipped (no attachment). 0 failed. 12 already seen.
```

Or if nothing new:
```
No new invoices found.
```

Exit code is `1` if any uploads failed (so they will be retried on the next run). Re-uploads are safe: the intake deduplicates identical files by hash, so a retry won't create a duplicate draft. Emails with no PDF/image attachment (invoice inline or as a download link) are skipped and marked seen — check stderr for `[skip]` lines.

## Files

| File | Purpose | Committed |
|---|---|---|
| `config.yaml` | Office Hub base URL + API token | ❌ gitignored |
| `senders.txt` | Invoice sender addresses | ✅ |
| `credentials.json` | Google OAuth client secret | ❌ gitignored |
| `token.json` | Saved OAuth token (auto-created on first run) | ❌ gitignored |
| `memory.json` | Processed message IDs (auto-created) | ❌ gitignored |

## Notes

- Run `./invoice-bot` from the project root — all config files are resolved relative to the working directory.
- To reset processed history, delete `memory.json` (all matching invoices will be uploaded again; the intake dedups, so no duplicate drafts).
- To re-authenticate, delete `token.json` and run again.
