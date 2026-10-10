/*
 * Copyright (c) 2026 Immanent Tech
 * SPDX-License-Identifier: AGPL-3.0-or-later
 */

package imgproxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// minSignatureBytes is the shortest (truncated) signature accepted. imgproxy's default is the full 32 bytes; a
// very short signature is trivially guessable.
const minSignatureBytes = 8

// NewSignatureVerifier returns a SignatureVerifier compatible with imgproxy's URL signing:
//
//	signature = base64url_raw(HMAC-SHA256(key, salt || path))
//
// keyHex and saltHex are the hex-encoded values of IMGPROXY_KEY and IMGPROXY_SALT. path is the part of the URL
// after the signature, including the leading slash. Truncated signatures (IMGPROXY_SIGNATURE_SIZE) are supported.
func NewSignatureVerifier(keyHex, saltHex string) (SignatureVerifier, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}
	if len(key) == 0 {
		return nil, errors.New("signing key is empty")
	}
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return nil, fmt.Errorf("decode salt: %w", err)
	}

	return func(signature, path string) bool {
		got, err := base64.RawURLEncoding.DecodeString(signature)
		if err != nil || len(got) < minSignatureBytes || len(got) > sha256.Size {
			return false
		}
		mac := hmac.New(sha256.New, key)
		mac.Write(salt)
		mac.Write([]byte(path))
		// Compare against the same-length prefix of the full MAC, in constant time.
		return hmac.Equal(got, mac.Sum(nil)[:len(got)])
	}, nil
}
