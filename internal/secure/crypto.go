package secure

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Cipher struct {
	aead cipher.AEAD
}

func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("master key must be exactly 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// LoadOrCreateKey loads a raw 32-byte key from path. On the first start it
// creates the parent directory and generates the key with owner-only access.
func LoadOrCreateKey(path string) ([]byte, bool, error) {
	if key, err := readKeyFile(path); err == nil {
		return key, false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, false, fmt.Errorf("create master key directory: %w", err)
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, false, fmt.Errorf("generate master key: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".master-key-*")
	if err != nil {
		return nil, false, fmt.Errorf("create temporary master key file: %w", err)
	}
	tempPath := file.Name()
	removeOnError := true
	defer func() {
		_ = file.Close()
		if removeOnError {
			_ = os.Remove(tempPath)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return nil, false, fmt.Errorf("set master key permissions: %w", err)
	}
	if _, err := file.Write(key); err != nil {
		return nil, false, fmt.Errorf("write master key file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return nil, false, fmt.Errorf("sync master key file: %w", err)
	}
	if err := file.Close(); err != nil {
		return nil, false, fmt.Errorf("close master key file: %w", err)
	}
	if err := os.Link(tempPath, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			winner, readErr := readKeyFile(path)
			if readErr != nil {
				return nil, false, readErr
			}
			return winner, false, nil
		}
		return nil, false, fmt.Errorf("publish master key file: %w", err)
	}
	if err := os.Remove(tempPath); err != nil {
		return nil, false, fmt.Errorf("remove temporary master key file: %w", err)
	}
	removeOnError = false
	return key, true, nil
}

func readKeyFile(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("read master key file: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("master key file %s must contain exactly 32 bytes", path)
	}
	return key, nil
}

func (c *Cipher) Encrypt(plaintext string) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (c *Cipher) Decrypt(ciphertext []byte) (string, error) {
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("encrypted value is too short")
	}
	nonce, payload := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt credentials: %w", err)
	}
	return string(plaintext), nil
}
