package publishing

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestAESGCMRefreshTokenProtectorRoundTripAndBinding(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	protector, err := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": key})
	if err != nil {
		t.Fatalf("new protector: %v", err)
	}
	ownerID := uuid.New()
	connectionID := uuid.New()
	envelope, err := protector.Protect(ownerID, connectionID, "refresh-token-secret")
	if err != nil {
		t.Fatalf("protect: %v", err)
	}
	if envelope.KeyID != "v1" || len(envelope.Ciphertext) == 0 || len(envelope.Nonce) == 0 {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	if string(envelope.Ciphertext) == "refresh-token-secret" {
		t.Fatal("refresh token stored as plaintext")
	}
	revealed, err := protector.Reveal(ownerID, connectionID, envelope)
	if err != nil {
		t.Fatalf("reveal: %v", err)
	}
	if revealed != "refresh-token-secret" {
		t.Fatalf("revealed token = %q", revealed)
	}
	if _, err := protector.Reveal(uuid.New(), connectionID, envelope); !errors.Is(err, ErrTokenDecryptFailed) {
		t.Fatalf("cross-owner reveal error = %v, want ErrTokenDecryptFailed", err)
	}
	if _, err := protector.Reveal(ownerID, uuid.New(), envelope); !errors.Is(err, ErrTokenDecryptFailed) {
		t.Fatalf("cross-connection reveal error = %v, want ErrTokenDecryptFailed", err)
	}
}

func TestAESGCMRefreshTokenProtectorSupportsKeyRotation(t *testing.T) {
	oldKey := []byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	newKey := []byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	ownerID := uuid.New()
	connectionID := uuid.New()
	oldProtector, err := NewAESGCMRefreshTokenProtector("old", map[string][]byte{"old": oldKey})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := oldProtector.Protect(ownerID, connectionID, "rotatable-token")
	if err != nil {
		t.Fatal(err)
	}
	rotatedProtector, err := NewAESGCMRefreshTokenProtector("new", map[string][]byte{"old": oldKey, "new": newKey})
	if err != nil {
		t.Fatal(err)
	}
	revealed, err := rotatedProtector.Reveal(ownerID, connectionID, envelope)
	if err != nil || revealed != "rotatable-token" {
		t.Fatalf("reveal after rotation = %q, %v", revealed, err)
	}
	newEnvelope, err := rotatedProtector.Protect(ownerID, uuid.New(), "new-token")
	if err != nil {
		t.Fatal(err)
	}
	if newEnvelope.KeyID != "new" {
		t.Fatalf("new envelope key = %q, want new", newEnvelope.KeyID)
	}
}

func TestAESGCMRefreshTokenProtectorRejectsInvalidKeysAndTampering(t *testing.T) {
	if _, err := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": []byte("short")}); !errors.Is(err, ErrTokenKeyUnavailable) {
		t.Fatalf("invalid key error = %v", err)
	}
	protector, err := NewAESGCMRefreshTokenProtector("v1", map[string][]byte{"v1": []byte("0123456789abcdef0123456789abcdef")})
	if err != nil {
		t.Fatal(err)
	}
	ownerID := uuid.New()
	connectionID := uuid.New()
	envelope, err := protector.Protect(ownerID, connectionID, "secret")
	if err != nil {
		t.Fatal(err)
	}
	envelope.Ciphertext[0] ^= 0xff
	if _, err := protector.Reveal(ownerID, connectionID, envelope); !errors.Is(err, ErrTokenDecryptFailed) {
		t.Fatalf("tamper error = %v, want ErrTokenDecryptFailed", err)
	}
}
