package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/banking/transaction-service/internal/config"
)

// EncryptionService handles encryption and decryption of sensitive data
type EncryptionService struct {
	keys map[int][]byte
	currentVersion int
}

// NewEncryptionService creates a new encryption service
func NewEncryptionService(cfg config.EncryptionConfig) (*EncryptionService, error) {
	// Parse base64 encoded keys
	// Format: "1:base64key1,2:base64key2"
	keyMap := make(map[int][]byte)
	parts := strings.Split(cfg.EncryptionKeysBase64, ",")
	for _, part := range parts {
		kp := strings.Split(part, ":")
		if len(kp) != 2 {
			continue // Skip malformed keys
		}
		var version int
		if _, err := fmt.Sscanf(kp[0], "%d", &version); err != nil {
			return nil, fmt.Errorf("invalid key version: %w", err)
		}
		keyBytes, err := base64.StdEncoding.DecodeString(kp[1])
		if err != nil {
			return nil, fmt.Errorf("invalid key encoding for version %d: %w", version, err)
		}
		if len(keyBytes) != 32 {
			return nil, fmt.Errorf("invalid key length for version %d: want 32 bytes, got %d", version, len(keyBytes))
		}
		keyMap[version] = keyBytes
	}

	if _, ok := keyMap[cfg.CurrentKeyVersion]; !ok {
		return nil, fmt.Errorf("current key version %d not found in provided keys", cfg.CurrentKeyVersion)
	}

	return &EncryptionService{
		keys:           keyMap,
		currentVersion: cfg.CurrentKeyVersion,
	}, nil
}

// Encrypt encrypts plaintext using the current key version
// Returns: version:nonce:ciphertext (base64 encoded)
func (s *EncryptionService) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	block, err := aes.NewCipher(s.keys[s.currentVersion])
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	
	// Format: version:base64(ciphertext)
	// Note: ciphertext includes nonce at the beginning due to Seal appending it
	return fmt.Sprintf("%d:%s", s.currentVersion, base64.StdEncoding.EncodeToString(ciphertext)), nil
}

// Decrypt decrypts the ciphertext
// Input format: version:base64(nonce+ciphertext)
func (s *EncryptionService) Decrypt(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	parts := strings.SplitN(encrypted, ":", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid encrypted data format")
	}

	var version int
	if _, err := fmt.Sscanf(parts[0], "%d", &version); err != nil {
		return "", fmt.Errorf("invalid version in encrypted data: %w", err)
	}

	key, ok := s.keys[version]
	if !ok {
		return "", fmt.Errorf("decryption key version %d not found", version)
	}

	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(data) < gcm.NonceSize() {
		return "", errors.New("malformed ciphertext")
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
