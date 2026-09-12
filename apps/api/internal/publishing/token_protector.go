package publishing

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

var (
	ErrTokenKeyUnavailable = errors.New("publishing token key unavailable")
	ErrTokenDecryptFailed  = errors.New("publishing token decrypt failed")
)

type RefreshTokenProtector interface {
	Protect(ownerID, connectionID uuid.UUID, refreshToken string) (RefreshTokenEnvelope, error)
	Reveal(ownerID, connectionID uuid.UUID, envelope RefreshTokenEnvelope) (string, error)
}

type AESGCMRefreshTokenProtector struct {
	currentKeyID string
	keys         map[string][]byte
	random       io.Reader
}

func NewAESGCMRefreshTokenProtector(currentKeyID string, keys map[string][]byte) (*AESGCMRefreshTokenProtector, error) {
	currentKeyID = strings.TrimSpace(currentKeyID)
	if currentKeyID == "" || len(keys) == 0 {
		return nil, ErrTokenKeyUnavailable
	}

	copied := make(map[string][]byte, len(keys))
	for keyID, key := range keys {
		keyID = strings.TrimSpace(keyID)
		if keyID == "" || len(key) != 32 {
			return nil, fmt.Errorf("%w: AES-256 key %q must be 32 bytes", ErrTokenKeyUnavailable, keyID)
		}
		copied[keyID] = append([]byte(nil), key...)
	}
	if _, ok := copied[currentKeyID]; !ok {
		return nil, fmt.Errorf("%w: current key %q missing", ErrTokenKeyUnavailable, currentKeyID)
	}
	return &AESGCMRefreshTokenProtector{currentKeyID: currentKeyID, keys: copied, random: rand.Reader}, nil
}

func (p *AESGCMRefreshTokenProtector) Protect(ownerID, connectionID uuid.UUID, refreshToken string) (RefreshTokenEnvelope, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if ownerID == uuid.Nil || connectionID == uuid.Nil || refreshToken == "" {
		return RefreshTokenEnvelope{}, ErrInvalidModel
	}
	block, err := aes.NewCipher(p.keys[p.currentKeyID])
	if err != nil {
		return RefreshTokenEnvelope{}, fmt.Errorf("create publishing token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return RefreshTokenEnvelope{}, fmt.Errorf("create publishing token gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(p.random, nonce); err != nil {
		return RefreshTokenEnvelope{}, fmt.Errorf("generate publishing token nonce: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(refreshToken), tokenAAD(ownerID, connectionID))
	return RefreshTokenEnvelope{Ciphertext: ciphertext, Nonce: nonce, KeyID: p.currentKeyID}, nil
}

func (p *AESGCMRefreshTokenProtector) Reveal(ownerID, connectionID uuid.UUID, envelope RefreshTokenEnvelope) (string, error) {
	if ownerID == uuid.Nil || connectionID == uuid.Nil || len(envelope.Ciphertext) == 0 || len(envelope.Nonce) == 0 || strings.TrimSpace(envelope.KeyID) == "" {
		return "", ErrInvalidModel
	}
	key, ok := p.keys[envelope.KeyID]
	if !ok {
		return "", ErrTokenKeyUnavailable
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create publishing token cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create publishing token gcm: %w", err)
	}
	if len(envelope.Nonce) != gcm.NonceSize() {
		return "", ErrTokenDecryptFailed
	}
	plaintext, err := gcm.Open(nil, envelope.Nonce, envelope.Ciphertext, tokenAAD(ownerID, connectionID))
	if err != nil {
		return "", ErrTokenDecryptFailed
	}
	if len(plaintext) == 0 {
		return "", ErrTokenDecryptFailed
	}
	return string(plaintext), nil
}

func tokenAAD(ownerID, connectionID uuid.UUID) []byte {
	return []byte("synvideo:publishing:refresh-token:" + ownerID.String() + ":" + connectionID.String())
}
