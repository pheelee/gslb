package crypto

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testKey() []byte {
	key := make([]byte, 32)
	_, _ = rand.Read(key)
	return key
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	enc, err := NewEncryptor(testKey())
	require.NoError(t, err)

	plaintext := `{"api_token":"super-secret-token","zone_id":"abc123"}`
	ciphertext, err := enc.Encrypt(plaintext)
	require.NoError(t, err)

	assert.True(t, IsEncrypted(ciphertext))
	assert.NotEqual(t, plaintext, ciphertext)

	decrypted, err := enc.Decrypt(ciphertext)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecrypt_PlaintextPassthrough(t *testing.T) {
	enc, err := NewEncryptor(testKey())
	require.NoError(t, err)

	plain := `{"api_token":"not-encrypted"}`
	result, err := enc.Decrypt(plain)
	require.NoError(t, err)
	assert.Equal(t, plain, result)
}

func TestNilEncryptor_Passthrough(t *testing.T) {
	var enc *Encryptor

	val := `{"api_token":"test"}`
	encrypted, err := enc.Encrypt(val)
	require.NoError(t, err)
	assert.Equal(t, val, encrypted)

	decrypted, err := enc.Decrypt(val)
	require.NoError(t, err)
	assert.Equal(t, val, decrypted)
}

func TestNewEncryptor_WrongKeyLength(t *testing.T) {
	_, err := NewEncryptor([]byte("too-short"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestEncrypt_UniqueCiphertexts(t *testing.T) {
	enc, err := NewEncryptor(testKey())
	require.NoError(t, err)

	plain := "same-plaintext"
	c1, _ := enc.Encrypt(plain)
	c2, _ := enc.Encrypt(plain)
	assert.NotEqual(t, c1, c2, "each encryption should produce unique ciphertext due to random nonce")
}

func TestDecrypt_WrongKey(t *testing.T) {
	enc1, _ := NewEncryptor(testKey())
	enc2, _ := NewEncryptor(testKey())

	ciphertext, _ := enc1.Encrypt("secret")
	_, err := enc2.Decrypt(ciphertext)
	assert.Error(t, err)
}
