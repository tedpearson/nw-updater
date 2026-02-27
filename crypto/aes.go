package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// EncryptAES256GCM encrypts plaintext with AES-GCM using keyString, returning base64(nonce|ciphertext).
func EncryptAES256GCM(plaintext []byte, keyString string) ([]byte, error) {
	key := keyFromString(keyString)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("unable to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("unable to create cipher: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("unable to generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAES256GCM decrypts base64(nonce|ciphertext) produced by EncryptAES256GCM.
func DecryptAES256GCM(encrypted []byte, keyString string) (string, error) {
	key := keyFromString(keyString)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("unable to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("unable to create cipher: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(encrypted) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("unable to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// keyFromString generates a 32-byte key from a string using SHA-256.
func keyFromString(keyString string) []byte {
	sum := sha256.Sum256([]byte(keyString))
	return sum[:]
}
