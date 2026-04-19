# Trafilatura Extractor Service

Extract main content from any webpage via a signed token API. No Cloudflare Worker required.

---

## How it works

```
Client
  1. Build payload:  "<url>|<format>|<unix_timestamp>"
  2. Sign:           HMAC-SHA256(SECRET_KEY + SECRET_SALT, payload)
  3. Encode token:   base64url(payload) + "." + base64url(signature)
  4. Call API:       GET /extract?token=<token>
                  or POST /extract  {"token": "<token>"}
        │
        ▼
FastAPI Service
  • Verify HMAC-SHA256 signature
  • Check token age (default TTL: 5 min)
  • Validate URL + format
  • Run Trafilatura extraction
  • Return content + metadata
```

---

## Quick start

```bash
# Install
pip install -r requirements.txt

# Run (SECRET_KEY and SECRET_SALT are required)
EXTRACTOR_KEY=my-key EXTRACTOR_SALT=my-salt uvicorn main:app --reload
```

### Generate a token

```bash
python token_generator.py \
  --url "https://en.wikipedia.org/wiki/Web_scraping" \
  --format markdown \
  --key "my-key" \
  --salt "my-salt"
```

Or use env vars so you don't repeat them:

```bash
export EXTRACTOR_KEY=my-key
export EXTRACTOR_SALT=my-salt
python token_generator.py --url "https://example.com/article" --format json
```

### Call the API

```bash
# GET
curl "http://localhost:7000/extract?token=<token>"

# POST
curl -X POST http://localhost:7000/extract \
  -H "Content-Type: application/json" \
  -d '{"token": "<token>"}'
```

---

## Docker

```bash
docker build -t trafilatura-extractor .

docker run -p 7000:7000 \
  -e EXTRACTOR_KEY=my-key \
  -e EXTRACTOR_SALT=my-salt \
  trafilatura-extractor
```

---

## API reference

### `GET /extract?token=<token>`
### `POST /extract`  `{ "token": "<token>" }`

**Success (200)**
```json
{
  "url": "https://example.com/article",
  "format": "markdown",
  "content": "# Article Title\n\nExtracted body...",
  "metadata": {
    "title": "Article Title",
    "author": "Jane Doe",
    "date": "2024-04-01",
    "description": "...",
    "sitename": "example.com",
    "categories": [],
    "tags": []
  },
  "extracted_at": 1713400000
}
```

**Errors**

| Status | Meaning                                          |
| ------ | ------------------------------------------------ |
| `400`  | Invalid format or non-http(s) URL                |
| `401`  | Bad signature, expired token, or malformed token |
| `422`  | Page fetched but no content could be extracted   |

### `GET /health`
```json
{ "status": "ok" }
```

---

## Token format

```
token = base64url(payload) + "." + base64url(signature)

payload   = "<url>|<format>|<unix_timestamp>"
signature = HMAC-SHA256(SECRET_KEY + SECRET_SALT, payload_bytes)
```

Tokens are replay-resistant via the embedded timestamp and server-side TTL check.

---

## Environment variables

| Variable            | Required | Default | Description                      |
| ------------------- | -------- | ------- | -------------------------------- |
| `EXTRACTOR_KEY`     | ✅        | —       | HMAC signing key                 |
| `EXTRACTOR_SALT`    | ✅        | —       | Extra entropy mixed with the key |
| `TOKEN_TTL_SECONDS` | no       | `300`   | Token lifetime in seconds        |

---

## Supported output formats

| Format     | Description                                   |
| ---------- | --------------------------------------------- |
| `markdown` | Clean Markdown — best for LLM ingestion       |
| `txt`      | Plain text, no markup                         |
| `html`     | Cleaned HTML fragment                         |
| `xml`      | TEI XML with structural annotations           |
| `json`     | Full extraction with text + metadata combined |
