"""
Trafilatura Extractor Service
─────────────────────────────
Extracts main content from webpages using Trafilatura.
Accepts signed tokens directly — no Cloudflare Worker required.

Endpoints:
  GET  /extract?token=<token>          → extract content
  POST /extract  { "token": "<token>" } → extract content
  GET  /health                          → { "status": "ok" }

Token structure (before encoding):
  "<url>|<format>|<unix_timestamp>"
  e.g. "https://example.com/article|markdown|1713400000"

Encoding:
  payload   = "<url>|<format>|<timestamp>"
  signature = HMAC-SHA256(SECRET_KEY + SECRET_SALT, payload)
  token     = base64url(payload) + "." + base64url(signature)

Environment variables:
  EXTRACTOR_KEY          - HMAC signing key (required)
  EXTRACTOR_SALT         - Extra entropy mixed into the signing key (required)
  EXTRACTOR_TOKEN_TTL_SECONDS   - Token lifetime in seconds (default: 300)
"""

import base64
import hashlib
import hmac
import json
import os
import time
import subprocess
from typing import Literal

import trafilatura
from trafilatura.settings import use_config
from fastapi import FastAPI, HTTPException, Query, Request
from fastapi.responses import JSONResponse
from pydantic import BaseModel

# ── Config ────────────────────────────────────────────────────────────────────

SECRET_KEY = os.environ.get("EXTRACTOR_KEY", "")
SECRET_SALT = os.environ.get("EXTRACTOR_SALT", "")
TOKEN_TTL_SECONDS = int(os.environ.get("EXTRACTOR_TOKEN_TTL_SECONDS", "300"))
VERSION = os.environ.get("EXTRACTOR_VERSION", "_UNKNOWN_")

if not VERSION or VERSION == "_UNKNOWN_":
    raise RuntimeError("APPVERSION environment variable is required")
if not SECRET_KEY:
    raise RuntimeError("EXTRACTOR_KEY environment variable is required")
if not SECRET_SALT:
    raise RuntimeError("EXTRACTOR_SALT environment variable is required")

# Trafilatura config
trafilatura_config = use_config()
trafilatura_config.set("DEFAULT", "EXTRACTION_TIMEOUT", "30")

# ── App ───────────────────────────────────────────────────────────────────────

app = FastAPI(
    title="Foragd Content Extractor",
    description="Web content extraction via signed tokens using trafilatura",
    version=VERSION,
)

# ── Token verification ────────────────────────────────────────────────────────

def _b64url_decode(s: str) -> bytes:
    """Decode a base64url string (no padding required)."""
    padding = 4 - len(s) % 4
    s += "=" * (padding % 4)
    return base64.urlsafe_b64decode(s)


def verify_and_decode_token(token: str) -> dict:
    """
    Verify HMAC-SHA256 signature and return decoded payload fields.
    Raises HTTPException on any failure.
    """
    parts = token.split(".")
    if len(parts) != 2:
        raise HTTPException(status_code=401, detail="Malformed token")

    payload_b64, sig_b64 = parts

    try:
        payload_bytes = _b64url_decode(payload_b64)
        received_sig = _b64url_decode(sig_b64)
    except Exception:
        raise HTTPException(status_code=401, detail="Token decode error")

    # Recompute expected signature
    key_material = (SECRET_KEY + SECRET_SALT).encode()
    expected_sig = hmac.new(key_material, payload_bytes, hashlib.sha256).digest()

    if not hmac.compare_digest(expected_sig, received_sig):
        raise HTTPException(status_code=401, detail="Invalid token signature")

    # Parse payload
    try:
        payload_str = payload_bytes.decode()
        fields = payload_str.split("|")
        if len(fields) != 3:
            raise ValueError("wrong field count")
        url, fmt, ts_str = fields
        timestamp = int(ts_str)
    except Exception:
        raise HTTPException(status_code=401, detail="Token payload malformed")

    # Check expiry
    age = int(time.time()) - timestamp
    if age > TOKEN_TTL_SECONDS:
        raise HTTPException(
            status_code=401,
            detail=f"Token expired (age: {age}s, max: {TOKEN_TTL_SECONDS}s)",
        )
    if age < -60:
        # Reject tokens dated more than 60s in the future (clock skew guard)
        raise HTTPException(status_code=401, detail="Token timestamp is in the future")

    return {"url": url, "format": fmt, "timestamp": timestamp}


# ── Models ────────────────────────────────────────────────────────────────────

ALLOWED_FORMATS = {"markdown", "txt", "html", "xml", "json"}

OutputFormat = Literal["markdown", "txt", "html", "xml", "json"]


class TokenRequest(BaseModel):
    token: str


class ExtractResponse(BaseModel):
    url: str
    format: str
    content: str | None
    metadata: dict | None
    extracted_at: int


# ── Extraction logic ──────────────────────────────────────────────────────────

def run_extraction(url: str, fmt: str) -> ExtractResponse:
    # Validate format
    if fmt not in ALLOWED_FORMATS:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid format '{fmt}'. Allowed: {', '.join(sorted(ALLOWED_FORMATS))}",
        )

    # Validate URL scheme
    if not url.startswith(("http://", "https://")):
        raise HTTPException(status_code=400, detail="Only http/https URLs are allowed")

    # Fetch page
    downloaded = trafilatura.fetch_url(url)
    if not downloaded:
        raise HTTPException(status_code=422, detail=f"Could not fetch URL: {url}")

    metadata = None
    content = None

    if fmt == "markdown":
        content = trafilatura.extract(
            downloaded,
            output_format="markdown",
            include_comments=False,
            include_tables=True,
            favor_precision=True,
            config=trafilatura_config,
        )

    elif fmt == "txt":
        content = trafilatura.extract(
            downloaded,
            output_format="txt",
            include_comments=False,
            include_tables=True,
            favor_precision=True,
            config=trafilatura_config,
        )

    elif fmt == "html":
        content = trafilatura.extract(
            downloaded,
            output_format="html",
            include_comments=False,
            include_tables=True,
            config=trafilatura_config,
        )

    elif fmt == "xml":
        content = trafilatura.extract(
            downloaded,
            output_format="xmltei",
            include_comments=False,
            include_tables=True,
            config=trafilatura_config,
        )

    elif fmt == "json":
        raw = trafilatura.extract(
            downloaded,
            output_format="json",
            include_comments=False,
            include_tables=True,
            favor_precision=True,
            config=trafilatura_config,
        )
        if raw:
            try:
                parsed = json.loads(raw)
                metadata = {k: v for k, v in parsed.items() if k != "text"}
                content = parsed.get("text")
            except json.JSONDecodeError:
                content = raw

    if content is None:
        raise HTTPException(
            status_code=422,
            detail="Trafilatura could not extract meaningful content from this URL.",
        )

    # Fetch metadata for non-JSON formats
    if metadata is None:
        meta = trafilatura.extract_metadata(downloaded)
        if meta:
            metadata = {
                "title": meta.title,
                "author": meta.author,
                "date": meta.date,
                "description": meta.description,
                "sitename": meta.sitename,
                "categories": meta.categories,
                "tags": meta.tags,
            }

    return ExtractResponse(
        url=url,
        format=fmt,
        content=content,
        metadata=metadata,
        extracted_at=int(time.time()),
    )


# ── Routes ────────────────────────────────────────────────────────────────────

@app.get("/health")
def health():
    return {"status": "ok"}


@app.get("/extract", response_model=ExtractResponse)
def extract_get(token: str = Query(..., description="Signed token")):
    payload = verify_and_decode_token(token)
    return run_extraction(payload["url"], payload["format"])


@app.post("/extract", response_model=ExtractResponse)
def extract_post(body: TokenRequest):
    payload = verify_and_decode_token(body.token)
    return run_extraction(payload["url"], payload["format"])
