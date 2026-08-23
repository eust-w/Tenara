package local

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

const (
	sealVersionV1 = byte(0x01)
	gcmNonceSize  = 12
	aes256KeySize = 32
)

// ErrSealMismatch marks authentication failure while opening a sealed
// payload: wrong master key or corrupted ciphertext.
var ErrSealMismatch = errors.New("sealed payload fails authentication")

// Sealer is the KMS-stub primitive: AES-256-GCM with a one-byte version
// header enabling future key-rotation re-encryption.
type Sealer struct {
	masterKey []byte
}

// NewSealer validates and copies the 32-byte AES-256 master key.
func NewSealer(masterKey []byte) (*Sealer, error) {
	if len(masterKey) != aes256KeySize {
		return nil, fmt.Errorf("master key must be %d bytes (AES-256), got %d",
			aes256KeySize, len(masterKey))
	}
	key := make([]byte, aes256KeySize)
	copy(key, masterKey)
	return &Sealer{masterKey: key}, nil
}

func (s *Sealer) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Encrypt emits version(1B) || nonce(12B) || ciphertext+tag.
func (s *Sealer) Encrypt(plaintext []byte) ([]byte, error) {
	aead, err := s.aead()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcmNonceSize)
	if _, nErr := rand.Read(nonce); nErr != nil {
		return nil, fmt.Errorf("nonce: %w", nErr)
	}
	out := make([]byte, 0, 1+gcmNonceSize+len(plaintext)+aead.Overhead())
	out = append(out, sealVersionV1)
	out = append(out, nonce...)
	return aead.Seal(out, nonce, plaintext, nil), nil
}

// Decrypt verifies the version header and opens the authenticated payload.
func (s *Sealer) Decrypt(sealed []byte) ([]byte, error) {
	header := 1 + gcmNonceSize
	if len(sealed) < header {
		return nil, fmt.Errorf("sealed payload too short (%d bytes)", len(sealed))
	}
	if sealed[0] != sealVersionV1 {
		return nil, fmt.Errorf("unsupported seal version %#x", sealed[0])
	}
	aead, err := s.aead()
	if err != nil {
		return nil, err
	}
	plain, openErr := aead.Open(nil, sealed[1:header], sealed[header:], nil)
	if openErr != nil {
		return nil, ErrSealMismatch
	}
	return plain, nil
}
