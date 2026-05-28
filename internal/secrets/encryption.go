package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	masterKeyBytes = 32
	hkdfInfo       = "credential-manager/local-store/v1"
	saltBytes      = 32
)

func decodeMasterKeyHex(masterKeyHex string) ([]byte, error) {
	decoded, err := hex.DecodeString(masterKeyHex)
	if err != nil {
		return nil, fmt.Errorf("master key must be valid hex: %w", err)
	}

	if len(decoded) != masterKeyBytes {
		return nil, fmt.Errorf("master key must decode to %d bytes", masterKeyBytes)
	}

	return decoded, nil
}

func deriveKey(masterKey []byte, salt []byte) ([]byte, error) {
	if len(masterKey) != masterKeyBytes {
		return nil, fmt.Errorf("master key must be %d bytes", masterKeyBytes)
	}

	h := hkdf.New(sha256.New, masterKey, salt, []byte(hkdfInfo))
	key := make([]byte, masterKeyBytes)
	if _, err := io.ReadFull(h, key); err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	return key, nil
}

func encryptSecret(masterKey []byte, plaintext string) (ciphertextB64 string, nonceB64 string, saltB64 string, err error) {
	salt := make([]byte, saltBytes)
	if _, err = rand.Read(salt); err != nil {
		return "", "", "", fmt.Errorf("generate salt: %w", err)
	}

	derivedKey, err := deriveKey(masterKey, salt)
	if err != nil {
		return "", "", "", err
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", "", "", fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", "", fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", "", "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), base64.StdEncoding.EncodeToString(salt), nil
}

func decryptSecret(masterKey []byte, ciphertextB64 string, nonceB64 string, saltB64 string) (string, error) {
	ciphertext, err := decodeBinary(ciphertextB64)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	nonce, err := decodeBinary(nonceB64)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}

	salt, err := decodeBinary(saltB64)
	if err != nil {
		return "", fmt.Errorf("decode salt: %w", err)
	}

	derivedKey, err := deriveKey(masterKey, salt)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return "", fmt.Errorf("create aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return "", fmt.Errorf("invalid nonce length: got %d want %d", len(nonce), gcm.NonceSize())
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt secret: %w", err)
	}

	return string(plaintext), nil
}

func decodeBinary(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}

	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}

	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}

	return nil, fmt.Errorf("value is neither base64 nor hex")
}
