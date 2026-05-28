package secrets

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mitchellh/mapstructure"
)

type SecretStore interface {
	LookupSecret(ctx context.Context, id string) (string, error)
	StoreSecret(ctx context.Context, id string, plaintext string) error
}

type LocalSecretStore struct {
	masterKey  []byte
	filePath   string
	autoCreate bool

	mu      sync.RWMutex
	secrets map[string]encryptedSecretRecord
}

type encryptedSecretRecord struct {
	Ciphertext string `json:"ciphertext" mapstructure:"ciphertext"`
	Nonce      string `json:"nonce" mapstructure:"nonce"`
	Salt       string `json:"salt" mapstructure:"salt"`
}

type localSecretsDocument struct {
	Secrets map[string]encryptedSecretRecord `json:"secrets"`
}

func NewLocalSecretStore(masterKeyHex string, filePath string, autoCreate bool) (*LocalSecretStore, error) {
	masterKey, err := decodeMasterKeyHex(masterKeyHex)
	if err != nil {
		return nil, err
	}

	if filePath == "" {
		return nil, fmt.Errorf("secrets file path must not be empty")
	}

	store := &LocalSecretStore{
		masterKey:  masterKey,
		filePath:   filePath,
		autoCreate: autoCreate,
		secrets:    map[string]encryptedSecretRecord{},
	}

	if err := store.ensureFile(); err != nil {
		return nil, err
	}
	if err := store.reloadFromDisk(); err != nil {
		return nil, err
	}

	return store, nil
}

func (s *LocalSecretStore) LookupSecret(ctx context.Context, id string) (string, error) {
	_ = ctx

	if err := s.reloadFromDisk(); err != nil {
		return "", err
	}

	s.mu.RLock()
	rec, ok := s.secrets[id]
	s.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("secret %q not found", id)
	}

	plaintext, err := decryptSecret(s.masterKey, rec.Ciphertext, rec.Nonce, rec.Salt)
	if err != nil {
		return "", fmt.Errorf("decrypt secret %q: %w", id, err)
	}

	return plaintext, nil
}

func (s *LocalSecretStore) StoreSecret(ctx context.Context, id string, plaintext string) error {
	_ = ctx

	ciphertextB64, nonceB64, saltB64, err := encryptSecret(s.masterKey, plaintext)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.secrets[id] = encryptedSecretRecord{
		Ciphertext: ciphertextB64,
		Nonce:      nonceB64,
		Salt:       saltB64,
	}
	s.mu.Unlock()

	return s.persistToDisk()
}

func (s *LocalSecretStore) ensureFile() error {
	if _, err := os.Stat(s.filePath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat secrets file: %w", err)
	}

	if !s.autoCreate {
		return fmt.Errorf("secrets file %q does not exist", s.filePath)
	}

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o700); err != nil {
		return fmt.Errorf("create secrets directory: %w", err)
	}

	doc := localSecretsDocument{Secrets: map[string]encryptedSecretRecord{}}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal initial secrets file: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0o600); err != nil {
		return fmt.Errorf("create secrets file: %w", err)
	}

	return nil
}

func (s *LocalSecretStore) reloadFromDisk() error {
	raw, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("read secrets file: %w", err)
	}

	secretsMap, err := decodeSecretsDocument(raw)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.secrets = secretsMap
	s.mu.Unlock()

	return nil
}

func (s *LocalSecretStore) persistToDisk() error {
	s.mu.RLock()
	copyMap := make(map[string]encryptedSecretRecord, len(s.secrets))
	for key, value := range s.secrets {
		copyMap[key] = value
	}
	s.mu.RUnlock()

	doc := localSecretsDocument{Secrets: copyMap}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal secrets file: %w", err)
	}

	tempPath := s.filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("write temp secrets file: %w", err)
	}

	if err := os.Rename(tempPath, s.filePath); err != nil {
		return fmt.Errorf("replace secrets file: %w", err)
	}

	return nil
}

func decodeSecretsDocument(raw []byte) (map[string]encryptedSecretRecord, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse secrets file json: %w", err)
	}

	candidate := root
	if nested, ok := root["secrets"]; ok {
		typed, ok := nested.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("secrets field must be an object")
		}
		candidate = typed
	}

	decoded := map[string]encryptedSecretRecord{}
	for key, value := range candidate {
		record, err := decodeSecretRecord(value)
		if err != nil {
			return nil, fmt.Errorf("decode secret %q: %w", key, err)
		}
		decoded[key] = record
	}

	return decoded, nil
}

func decodeSecretRecord(raw any) (encryptedSecretRecord, error) {
	var rec encryptedSecretRecord
	if err := mapstructure.Decode(raw, &rec); err != nil {
		return rec, err
	}

	if rec.Ciphertext == "" || rec.Nonce == "" || rec.Salt == "" {
		return rec, fmt.Errorf("record must include ciphertext, nonce, and salt")
	}

	return rec, nil
}
