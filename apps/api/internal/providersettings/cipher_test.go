package providersettings_test

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/providers"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/providersettings"
)

func generateTestKey(t *testing.T) string {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate random key: %v", err)
	}
	return hex.EncodeToString(key)
}

func TestCipher_RoundTripAndRandomNonce(t *testing.T) {
	keyHex := generateTestKey(t)
	cipher, err := providersettings.NewAESGCMCipher(keyHex, "v1")
	if err != nil {
		t.Fatalf("unexpected error creating cipher: %v", err)
	}

	ownerID := uuid.New()
	providerID := providers.ProviderID("openai")
	keyVersion := "v1"
	plaintext := "sk-test-secret-api-key-12345"

	// Encrypt twice with the same inputs
	c1, nonce1, err := cipher.Encrypt(ownerID, providerID, keyVersion, plaintext)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}
	c2, nonce2, err := cipher.Encrypt(ownerID, providerID, keyVersion, plaintext)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	// Nonces must be random and distinct
	if hex.EncodeToString(nonce1) == hex.EncodeToString(nonce2) {
		t.Fatalf("expected different nonces for consecutive encryptions, got identical: %x", nonce1)
	}
	if hex.EncodeToString(c1) == hex.EncodeToString(c2) {
		t.Fatalf("expected different ciphertexts for consecutive encryptions due to distinct nonces")
	}

	// Decrypt c1
	decrypted1, err := cipher.Decrypt(ownerID, providerID, keyVersion, c1, nonce1)
	if err != nil {
		t.Fatalf("decrypt c1 failed: %v", err)
	}
	if decrypted1 != plaintext {
		t.Fatalf("expected plaintext %q, got %q", plaintext, decrypted1)
	}

	// Decrypt c2
	decrypted2, err := cipher.Decrypt(ownerID, providerID, keyVersion, c2, nonce2)
	if err != nil {
		t.Fatalf("decrypt c2 failed: %v", err)
	}
	if decrypted2 != plaintext {
		t.Fatalf("expected plaintext %q, got %q", plaintext, decrypted2)
	}
}

func TestCipher_AADBindingAndTamperRejection(t *testing.T) {
	keyHex := generateTestKey(t)
	cipher, err := providersettings.NewAESGCMCipher(keyHex, "v1")
	if err != nil {
		t.Fatalf("unexpected error creating cipher: %v", err)
	}

	ownerA := uuid.New()
	ownerB := uuid.New()
	providerA := providers.ProviderID("openai")
	providerB := providers.ProviderID("anthropic")
	plaintext := "secret-sentinel-token-to-protect"

	ciphertext, nonce, err := cipher.Encrypt(ownerA, providerA, "v1", plaintext)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	t.Run("owner mismatch fails", func(t *testing.T) {
		_, err := cipher.Decrypt(ownerB, providerA, "v1", ciphertext, nonce)
		if !errors.Is(err, providersettings.ErrDecryptionFailed) {
			t.Fatalf("expected ErrDecryptionFailed on owner mismatch, got %v", err)
		}
	})

	t.Run("provider mismatch fails", func(t *testing.T) {
		_, err := cipher.Decrypt(ownerA, providerB, "v1", ciphertext, nonce)
		if !errors.Is(err, providersettings.ErrDecryptionFailed) {
			t.Fatalf("expected ErrDecryptionFailed on provider mismatch, got %v", err)
		}
	})

	t.Run("key version mismatch fails", func(t *testing.T) {
		_, err := cipher.Decrypt(ownerA, providerA, "v2", ciphertext, nonce)
		if !errors.Is(err, providersettings.ErrDecryptionFailed) {
			t.Fatalf("expected ErrDecryptionFailed on key version mismatch, got %v", err)
		}
	})

	t.Run("tampered ciphertext fails", func(t *testing.T) {
		tampered := make([]byte, len(ciphertext))
		copy(tampered, ciphertext)
		tampered[len(tampered)-1] ^= 0xFF

		_, err := cipher.Decrypt(ownerA, providerA, "v1", tampered, nonce)
		if !errors.Is(err, providersettings.ErrDecryptionFailed) {
			t.Fatalf("expected ErrDecryptionFailed on tampered ciphertext, got %v", err)
		}
	})

	t.Run("tampered nonce fails", func(t *testing.T) {
		tamperedNonce := make([]byte, len(nonce))
		copy(tamperedNonce, nonce)
		tamperedNonce[0] ^= 0xFF

		_, err := cipher.Decrypt(ownerA, providerA, "v1", ciphertext, tamperedNonce)
		if !errors.Is(err, providersettings.ErrDecryptionFailed) {
			t.Fatalf("expected ErrDecryptionFailed on tampered nonce, got %v", err)
		}
	})

	t.Run("error message never leaks plaintext secret", func(t *testing.T) {
		_, err := cipher.Decrypt(ownerB, providerA, "v1", ciphertext, nonce)
		if err != nil && strings.Contains(err.Error(), plaintext) {
			t.Fatalf("error message contains plaintext secret: %v", err)
		}
	})
}

func TestCipher_InvalidMasterKey(t *testing.T) {
	t.Run("empty key", func(t *testing.T) {
		_, err := providersettings.NewAESGCMCipher("", "v1")
		if err == nil {
			t.Fatal("expected error for empty key")
		}
	})

	t.Run("invalid length key", func(t *testing.T) {
		_, err := providersettings.NewAESGCMCipher("shortkey", "v1")
		if err == nil {
			t.Fatal("expected error for short key")
		}
	})

	t.Run("empty key version", func(t *testing.T) {
		keyHex := generateTestKey(t)
		_, err := providersettings.NewAESGCMCipher(keyHex, "")
		if err == nil {
			t.Fatal("expected error for empty key version")
		}
	})
}
