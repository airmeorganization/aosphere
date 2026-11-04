package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
)

// DeriveKey derives a 32-byte AES-256 key from a passphrase or raw key using SHA-256.
func DeriveKey(passphrase string) []byte {
	hash := sha256.Sum256([]byte(passphrase))
	return hash[:]
}

// EncryptFile encrypts srcPath using AES-256-GCM and writes the ciphertext to dstPath.
// The output file format is: [12-byte nonce] + [AES-256-GCM ciphertext + 16-byte auth tag].
func EncryptFile(srcPath, dstPath, passphrase string) error {
	if passphrase == "" {
		return errors.New("passphrase cannot be empty for encrypted backup")
	}

	key := DeriveKey(passphrase)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM block: %w", err)
	}

	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("failed to generate random nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	if err := os.WriteFile(dstPath, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write encrypted file: %w", err)
	}

	return nil
}

// DecryptFile decrypts srcPath using AES-256-GCM and writes the plaintext to dstPath.
func DecryptFile(srcPath, dstPath, passphrase string) error {
	if passphrase == "" {
		return errors.New("passphrase cannot be empty for decryption")
	}

	key := DeriveKey(passphrase)
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM block: %w", err)
	}

	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read encrypted file: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return errors.New("ciphertext too short: invalid encrypted backup format")
	}

	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return fmt.Errorf("decryption failed (invalid passphrase or corrupted file): %w", err)
	}

	if err := os.WriteFile(dstPath, plaintext, 0600); err != nil {
		return fmt.Errorf("failed to write decrypted file: %w", err)
	}

	return nil
}

// ComputeFileSHA256 returns the hexadecimal SHA-256 hash of a file.
func ComputeFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

