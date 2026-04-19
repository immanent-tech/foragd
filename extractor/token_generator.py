#!/usr/bin/env python3
"""
token_generator.py
──────────────────
Generate and decode signed tokens for the Trafilatura extractor service.

Usage:
  python token_generator.py \\
    --url "https://example.com/article" \\
    --format markdown \\
    --key "my-secret-key" \\
    --salt "my-salt"

Or set env vars EXTRACTOR_KEY / EXTRACTOR_SALT and omit --key / --salt.
"""

import argparse
import base64
import hashlib
import hmac
import os
import time
from urllib.parse import quote


def _b64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode()


def _b64url_decode(s: str) -> bytes:
    padding = 4 - len(s) % 4
    s += "=" * (padding % 4)
    return base64.urlsafe_b64decode(s)


def generate_token(url: str, fmt: str, key: str, salt: str, timestamp: int | None = None) -> str:
    """
    Build and sign a token.

    Token format:  base64url(payload) + "." + base64url(HMAC-SHA256(key+salt, payload))
    Payload:       "<url>|<format>|<unix_timestamp>"
    """
    ts = timestamp if timestamp is not None else int(time.time())
    payload = f"{url}|{fmt}|{ts}".encode()
    key_material = (key + salt).encode()
    sig = hmac.new(key_material, payload, hashlib.sha256).digest()
    return f"{_b64url_encode(payload)}.{_b64url_encode(sig)}"


def decode_token(token: str) -> dict | None:
    """
    Decode token payload WITHOUT verifying the signature.
    Useful for debugging only — never trust unverified data.
    """
    try:
        payload_b64, _ = token.split(".", 1)
        payload = _b64url_decode(payload_b64).decode()
        url, fmt, ts = payload.split("|")
        return {"url": url, "format": fmt, "timestamp": int(ts)}
    except Exception:
        return None


# ── CLI ───────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Generate a signed extractor token")
    parser.add_argument("--url",    required=True,               help="Target URL")
    parser.add_argument("--format", default="markdown",          help="Output format (default: markdown)")
    parser.add_argument("--key",    default=os.getenv("EXTRACTOR_KEY"),  help="Secret key")
    parser.add_argument("--salt",   default=os.getenv("EXTRACTOR_SALT"), help="Secret salt")
    args = parser.parse_args()

    if not args.key or not args.salt:
        parser.error("--key and --salt are required (or set SECRET_KEY / SECRET_SALT env vars)")

    token = generate_token(args.url, args.format, args.key, args.salt)
    decoded = decode_token(token)

    print("\n── Generated Token ──────────────────────────────────────────")
    print(token)
    print("\n── Decoded Payload ──────────────────────────────────────────")
    for k, v in decoded.items():
        print(f"  {k}: {v}")
    print("\n── Example Requests ─────────────────────────────────────────")
    print(f"  GET  http://localhost:7000/extract?token={quote(token)}")
    print(f"  POST http://localhost:7000/extract")
    print(f'       Body: {{"token": "{token}"}}')
    print()
