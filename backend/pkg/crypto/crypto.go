package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// EncryptString mã hóa plaintext bằng AES-256-GCM, trả về base64.
func EncryptString(key string, plaintext string) (string, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, []byte(plaintext), nil)

	out := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptString giải mã ciphertext base64 bằng AES-256-GCM.
func DecryptString(key string, encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	aead, err := newAEAD(key)
	if err != nil {
		return "", err
	}

	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func newAEAD(key string) (cipher.AEAD, error) {
	fullKey := deriveKey([]byte(key))
	block, err := aes.NewCipher(fullKey)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// deriveKey extends/truncates key to reach 32 bytes.
func deriveKey(key []byte) []byte {
	const size = 32
	if len(key) == size {
		return key
	}
	if len(key) > size {
		return key[:size]
	}
	out := make([]byte, size)
	copy(out, key)
	return out
}
