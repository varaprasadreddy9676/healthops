// Package cryptoutil provides shared AES-256-GCM encryption helpers for
// secrets stored at rest (MySQL passwords, SSH passwords, etc).
//
// The package-level key is loaded once at startup via Init(dataDir). The key
// file lives at <dataDir>/.secret_enc_key and is auto-generated on first run.
//
// Usage:
//
//	cryptoutil.Init("backend/data")
//	enc, _ := cryptoutil.Encrypt("my-password")
//	plain, _ := cryptoutil.Decrypt(enc)
package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// SecretMask is the placeholder shown in API responses where a secret exists.
// Clients can send this sentinel back on update to preserve the stored value.
const SecretMask = "********"

var (
	mu  sync.RWMutex
	key []byte
)

// Init loads or creates the encryption key at <dataDir>/.secret_enc_key.
// Safe to call multiple times; subsequent calls are no-ops.
func Init(dataDir string) error {
	mu.Lock()
	defer mu.Unlock()
	if key != nil {
		return nil
	}
	if dataDir == "" {
		return errors.New("cryptoutil: dataDir is empty")
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("cryptoutil: create data dir: %w", err)
	}
	path := filepath.Join(dataDir, ".secret_enc_key")
	k, err := loadOrCreate(path)
	if err != nil {
		return err
	}
	key = k
	return nil
}

// Ready reports whether Init has been called successfully.
func Ready() bool {
	mu.RLock()
	defer mu.RUnlock()
	return key != nil
}

// Encrypt returns a hex-encoded AES-256-GCM ciphertext of plaintext.
// Returns an error if Init has not been called.
func Encrypt(plaintext string) (string, error) {
	mu.RLock()
	k := key
	mu.RUnlock()
	if k == nil {
		return "", errors.New("cryptoutil: not initialized")
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ct), nil
}

// Decrypt returns the plaintext from a hex-encoded AES-256-GCM ciphertext.
func Decrypt(cipherHex string) (string, error) {
	mu.RLock()
	k := key
	mu.RUnlock()
	if k == nil {
		return "", errors.New("cryptoutil: not initialized")
	}
	raw, err := hex.DecodeString(cipherHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("cryptoutil: ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}

func loadOrCreate(path string) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		k, err := hex.DecodeString(strings.TrimSpace(string(data)))
		if err == nil && len(k) == 32 {
			return k, nil
		}
	}
	k := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return nil, fmt.Errorf("cryptoutil: generate key: %w", err)
	}
	encoded := hex.EncodeToString(k)
	if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
		return nil, fmt.Errorf("cryptoutil: save key: %w", err)
	}
	return k, nil
}
