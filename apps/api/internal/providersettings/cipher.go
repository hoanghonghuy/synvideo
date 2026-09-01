package providersettings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
)

// Cipher encrypts and decrypts credentials with authenticated additional data.
type Cipher interface {
	Encrypt(ownerID uuid.UUID, providerID providers.ProviderID, keyVersion string, plaintext string) (ciphertext []byte, nonce []byte, err error)
	Decrypt(ownerID uuid.UUID, providerID providers.ProviderID, keyVersion string, ciphertext []byte, nonce []byte) (string, error)
	KeyVersion() string
}

// AESGCMCipher implements Cipher using AES-256-GCM.
type AESGCMCipher struct {
	aead       cipher.AEAD
	keyVersion string
}

// NewAESGCMCipher creates a new AES-256-GCM cipher from a hex-encoded or 32-byte raw master key.
func NewAESGCMCipher(masterKey string, keyVersion string) (*AESGCMCipher, error) {
	if strings.TrimSpace(keyVersion) == "" {
		return nil, errors.New("key version is required")
	}

	keyBytes, err := decodeMasterKey(masterKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMasterKeyMissing, err)
	}

	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("%w: master key must be 32 bytes (got %d)", ErrMasterKeyMissing, len(keyBytes))
	}

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMasterKeyMissing, err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMasterKeyMissing, err)
	}

	return &AESGCMCipher{
		aead:       aead,
		keyVersion: keyVersion,
	}, nil
}

func (c *AESGCMCipher) KeyVersion() string {
	return c.keyVersion
}

func (c *AESGCMCipher) Encrypt(ownerID uuid.UUID, providerID providers.ProviderID, keyVersion string, plaintext string) ([]byte, []byte, error) {
	if keyVersion != c.keyVersion {
		return nil, nil, fmt.Errorf("%w: unexpected key version %q (expected %q)", ErrEncryptionFailed, keyVersion, c.keyVersion)
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("%w: failed to generate nonce: %v", ErrEncryptionFailed, err)
	}

	aad := computeAAD(ownerID, providerID, keyVersion)
	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), aad)

	return ciphertext, nonce, nil
}

func (c *AESGCMCipher) Decrypt(ownerID uuid.UUID, providerID providers.ProviderID, keyVersion string, ciphertext []byte, nonce []byte) (string, error) {
	if keyVersion != c.keyVersion {
		return "", ErrDecryptionFailed
	}

	if len(nonce) != c.aead.NonceSize() {
		return "", ErrDecryptionFailed
	}

	aad := computeAAD(ownerID, providerID, keyVersion)
	plaintextBytes, err := c.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	return string(plaintextBytes), nil
}

func computeAAD(ownerID uuid.UUID, providerID providers.ProviderID, keyVersion string) []byte {
	return []byte(ownerID.String() + ":" + string(providerID) + ":" + keyVersion)
}

func decodeMasterKey(keyStr string) ([]byte, error) {
	trimmed := strings.TrimSpace(keyStr)
	if trimmed == "" {
		return nil, errors.New("empty master key")
	}

	// Try hex decoding first if 64 characters
	if len(trimmed) == 64 {
		if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}

	// If exactly 32 raw bytes
	if len(trimmed) == 32 {
		return []byte(trimmed), nil
	}

	return nil, fmt.Errorf("master key must be 64-hex chars or 32 raw bytes (got %d chars)", len(trimmed))
}
