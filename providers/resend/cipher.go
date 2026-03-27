// Copyright 2026 Joshua Rich <joshua.rich@gmail.com>.
// SPDX-License-Identifier: 	AGPL-3.0-or-later

package resend

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

const (
	keyLen     = 32 // AES-256
	iterations = 100_000
)

// deriveKey uses PBKDF2 with SHA-256 to derive a fixed-length AES key from
// a passphrase and salt.
func deriveKey(passphrase, salt string) ([]byte, error) {
	return pbkdf2.Key(sha256.New, passphrase, []byte(salt), iterations, keyLen)
}

// EncodeEmail encrypts an email using AES-GCM with a key derived from the
// passphrase + salt. The result is URL-safe base64 (no padding).
func EncodeEmail(email string) (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	key, err := deriveKey(cfg.Key, cfg.Salt)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// Generate a random nonce for every encryption.
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Seal appends the ciphertext + auth tag to the nonce.
	ciphertext := aesGCM.Seal(nonce, nonce, []byte(email), nil)

	// URL-safe base64 without padding keeps the result URL-friendly.
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// DecodeEmail decrypts and returns the plain-text email encrypted by EncodeEmail.
func DecodeEmail(token string) (string, error) {
	if err := loadConfig(); err != nil {
		return "", fmt.Errorf("load config: %w", err)
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	key, err := deriveKey(cfg.Key, cfg.Salt)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("token too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Deliberately vague: wrong key/salt or tampered data.
		return "", errors.New("decryption failed: invalid token, key, or salt")
	}

	return string(plaintext), nil
}
