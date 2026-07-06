package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

type FieldEncryptor struct {
	gcm cipher.AEAD
}

func NewFieldEncryptor(keyB64 string) (*FieldEncryptor, error) {
	if keyB64 == "" {
		return nil, errors.New("DATABASE_ENCRYPTION_KEY is required")
	}
	key, err := decodeKey(keyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &FieldEncryptor{gcm: gcm}, nil
}

func decodeKey(keyB64 string) ([]byte, error) {
	decoders := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range decoders {
		if key, err := enc.DecodeString(keyB64); err == nil {
			if len(key) == 32 {
				return key, nil
			}
		}
	}
	return nil, errors.New("key must be 32 bytes after base64 decode")
}

func (e *FieldEncryptor) Encrypt(plaintext, aad string) (nonce, ciphertext []byte, err error) {
	nonce = make([]byte, e.gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = e.gcm.Seal(nil, nonce, []byte(plaintext), []byte(aad))
	return nonce, ciphertext, nil
}

func (e *FieldEncryptor) Decrypt(nonce, ciphertext []byte, aad string) (string, error) {
	plain, err := e.gcm.Open(nil, nonce, ciphertext, []byte(aad))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// PasswordFingerprint returns HMAC-SHA256(password, pepper) so a DB leak alone
// cannot be used for fast offline guessing without the server secret.
func PasswordFingerprint(password, pepper string) string {
	mac := hmac.New(sha256.New, []byte(pepper))
	_, _ = mac.Write([]byte(password))
	return hex.EncodeToString(mac.Sum(nil))
}

// LegacyPasswordFingerprint is the pre-pepper SHA-256 lookup key kept for migration.
func LegacyPasswordFingerprint(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}
