// Package kms is the local-first Key Management stub (RB-0.1): AES-256-GCM
// with a master key sourced from the environment. The baidu-* provider swaps
// this for real cloud KMS without changing call sites.
package kms

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

const masterKeyBytes = 32

var ErrBadMasterKey = errors.New("master key must be 64 hex chars (32 bytes)")

type Stub struct {
	aead cipher.AEAD
}

func NewStub(masterKeyHex string) (*Stub, error) {
	raw, decodeErr := hex.DecodeString(masterKeyHex)
	if decodeErr != nil || len(raw) != masterKeyBytes {
		return nil, ErrBadMasterKey
	}
	block, blockErr := aes.NewCipher(raw)
	if blockErr != nil {
		return nil, fmt.Errorf("new cipher: %w", blockErr)
	}
	aead, aeadErr := cipher.NewGCM(block)
	if aeadErr != nil {
		return nil, fmt.Errorf("new gcm: %w", aeadErr)
	}
	return &Stub{aead: aead}, nil
}

// Encrypt returns nonce||ciphertext.
func (s *Stub) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, randErr := rand.Read(nonce); randErr != nil {
		return nil, fmt.Errorf("read nonce: %w", randErr)
	}
	return s.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (s *Stub) Decrypt(sealed []byte) ([]byte, error) {
	if len(sealed) < s.aead.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, body := sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():]
	plain, openErr := s.aead.Open(nil, nonce, body, nil)
	if openErr != nil {
		return nil, fmt.Errorf("open: %w", openErr)
	}
	return plain, nil
}
