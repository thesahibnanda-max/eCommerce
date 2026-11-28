package credentialsRepository

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

const (
	keySalt  = "eCommerce credentials repository"
	keyInfo  = "aes-256-gcm"
	keyBytes = 32
)

func newAEAD(secret string) (cipher.AEAD, error) {
	key, err := hkdf.Key(sha256.New, []byte(secret), []byte(keySalt), keyInfo, keyBytes)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (r *repository) seal(name, secret string) ([]byte, error) {
	nonce := make([]byte, r.aead.NonceSize(), r.aead.NonceSize()+len(secret)+r.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("credentialsRepository: nonce: %w", err)
	}
	return r.aead.Seal(nonce, nonce, []byte(secret), []byte(name)), nil
}

func (r *repository) open(name string, sealed []byte) (string, error) {
	size := r.aead.NonceSize()
	if len(sealed) < size+r.aead.Overhead() {
		return "", ErrRecordCorrupted
	}
	plain, err := r.aead.Open(nil, sealed[:size], sealed[size:], []byte(name))
	if err != nil {
		return "", ErrDecryptFailed
	}
	return string(plain), nil
}
