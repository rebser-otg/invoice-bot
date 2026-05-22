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

On first run the tool opens your browser automatically (or prints the URL if it can't). Sign in with your Google Workspace account and grant access — the browser will show "Authorization successful" and the token is saved to `token.json`. Subsequent runs are fully automatic.

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
