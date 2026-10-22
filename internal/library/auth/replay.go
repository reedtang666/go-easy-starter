package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const refreshReplayKeyContext = "auth-refresh-replay:"

func sealRefreshReplay(refreshHashKey string, pair *TokenPair) (string, error) {
	if pair == nil {
		return "", errors.New("refresh replay pair is required")
	}
	payload, err := json.Marshal(pair)
	if err != nil {
		return "", fmt.Errorf("marshal refresh replay: %w", err)
	}
	aead, err := refreshReplayAEAD(refreshHashKey)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate refresh replay nonce: %w", err)
	}
	sealed := aead.Seal(nonce, nonce, payload, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func openRefreshReplay(refreshHashKey, encoded string) (*TokenPair, error) {
	aead, err := refreshReplayAEAD(refreshHashKey)
	if err != nil {
		return nil, err
	}
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode refresh replay: %w", err)
	}
	if len(sealed) < aead.NonceSize() {
		return nil, errors.New("refresh replay payload is too short")
	}
	nonce, ciphertext := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	payload, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt refresh replay: %w", err)
	}
	var pair TokenPair
	if err = json.Unmarshal(payload, &pair); err != nil {
		return nil, fmt.Errorf("decode refresh replay payload: %w", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		return nil, errors.New("refresh replay token pair is incomplete")
	}
	return &pair, nil
}

func refreshReplayAEAD(refreshHashKey string) (cipher.AEAD, error) {
	if refreshHashKey == "" {
		return nil, errors.New("refresh hash key is required")
	}
	key := sha256.Sum256([]byte(refreshReplayKeyContext + refreshHashKey))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("create refresh replay cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create refresh replay AEAD: %w", err)
	}
	return aead, nil
}
