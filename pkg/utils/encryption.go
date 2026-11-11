package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// DeriveStorageEncryptionKey derives a 32-byte AES-256 key from the given master secret and a
// domain separation string. The master secret lives in the server configuration file, which is
// outside the database, so data encrypted with the derived key cannot be recovered from the
// database alone
func DeriveStorageEncryptionKey(masterSecret string, domain string) []byte {
	hash := sha256.Sum256([]byte(domain + ":" + masterSecret))
	return hash[:]
}

// EncryptStringWithAESGCM encrypts the plaintext with AES-256-GCM and returns
// base64(nonce + ciphertext + tag). The additional data is authenticated but not encrypted, so
// ciphertexts cannot be swapped between different fields
func EncryptStringWithAESGCM(key []byte, additionalData string, plaintext string) (string, error) {
	gcm, err := newGcmBlock(key)

	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), []byte(additionalData))

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptStringWithAESGCM decrypts base64(nonce + ciphertext + tag) produced by
// EncryptStringWithAESGCM
func DecryptStringWithAESGCM(key []byte, additionalData string, encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)

	if err != nil {
		return "", err
	}

	gcm, err := newGcmBlock(key)

	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("encrypted data is too short")
	}

	plaintext, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], []byte(additionalData))

	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func newGcmBlock(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}
