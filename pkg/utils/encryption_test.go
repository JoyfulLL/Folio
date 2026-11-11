package utils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptStringWithAESGCM(t *testing.T) {
	key := DeriveStorageEncryptionKey("test-master-secret", "test-domain:v1")

	encrypted, err := EncryptStringWithAESGCM(key, "test-aad", "https://open.feishu.cn/open-apis/bot/v2/hook/secret-token")
	assert.Nil(t, err)
	assert.NotEmpty(t, encrypted)
	assert.NotContains(t, encrypted, "secret-token")

	decrypted, err := DecryptStringWithAESGCM(key, "test-aad", encrypted)
	assert.Nil(t, err)
	assert.Equal(t, "https://open.feishu.cn/open-apis/bot/v2/hook/secret-token", decrypted)
}

func TestEncryptStringWithAESGCMRandomNonce(t *testing.T) {
	key := DeriveStorageEncryptionKey("test-master-secret", "test-domain:v1")

	encrypted1, err1 := EncryptStringWithAESGCM(key, "aad", "same plaintext")
	encrypted2, err2 := EncryptStringWithAESGCM(key, "aad", "same plaintext")
	assert.Nil(t, err1)
	assert.Nil(t, err2)
	assert.NotEqual(t, encrypted1, encrypted2, "encrypting the same plaintext twice must produce different ciphertexts")
}

func TestDecryptStringWithAESGCMWithWrongKey(t *testing.T) {
	key := DeriveStorageEncryptionKey("test-master-secret", "test-domain:v1")
	wrongKey := DeriveStorageEncryptionKey("another-secret", "test-domain:v1")

	encrypted, err := EncryptStringWithAESGCM(key, "aad", "plaintext")
	assert.Nil(t, err)

	_, err = DecryptStringWithAESGCM(wrongKey, "aad", encrypted)
	assert.NotNil(t, err, "decrypting with a wrong key must fail")
}

func TestDecryptStringWithAESGCMWithWrongAdditionalData(t *testing.T) {
	key := DeriveStorageEncryptionKey("test-master-secret", "test-domain:v1")

	encrypted, err := EncryptStringWithAESGCM(key, "aad-1", "plaintext")
	assert.Nil(t, err)

	_, err = DecryptStringWithAESGCM(key, "aad-2", encrypted)
	assert.NotNil(t, err, "decrypting with wrong additional data must fail")
}

func TestDecryptStringWithAESGCMWithTamperedData(t *testing.T) {
	key := DeriveStorageEncryptionKey("test-master-secret", "test-domain:v1")

	encrypted, err := EncryptStringWithAESGCM(key, "aad", "plaintext")
	assert.Nil(t, err)

	tampered := []byte(encrypted)
	tampered[len(tampered)-2] ^= 0xff

	_, err = DecryptStringWithAESGCM(key, "aad", string(tampered))
	assert.NotNil(t, err, "decrypting tampered ciphertext must fail")
}

func TestDecryptStringWithAESGCMWithInvalidInput(t *testing.T) {
	key := DeriveStorageEncryptionKey("test-master-secret", "test-domain:v1")

	_, err := DecryptStringWithAESGCM(key, "aad", "not-base64!!!")
	assert.NotNil(t, err)

	_, err = DecryptStringWithAESGCM(key, "aad", "c21hbGw=")
	assert.NotNil(t, err, "decrypting data shorter than the nonce size must fail")
}

func TestDeriveStorageEncryptionKeyDifferentDomains(t *testing.T) {
	key1 := DeriveStorageEncryptionKey("secret", "domain-1")
	key2 := DeriveStorageEncryptionKey("secret", "domain-2")

	assert.False(t, bytes.Equal(key1, key2), "different domains must derive different keys")
}
