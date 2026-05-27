package secrets

import (
	"fmt"
	"os"
	"strings"
	"testing"

	infisical "github.com/infisical/go-sdk"
)

type mockInfisicalSecretsClient struct {
	retrieveResp infisical.Secret
	retrieveErr  error
	retrieveReq  infisical.RetrieveSecretOptions

	createErr error
	createReq infisical.CreateSecretOptions

	updateErr error
	updateReq infisical.UpdateSecretOptions

	deleteErr error
	deleteReq infisical.DeleteSecretOptions
}

func (m *mockInfisicalSecretsClient) Retrieve(options infisical.RetrieveSecretOptions) (infisical.Secret, error) {
	m.retrieveReq = options
	if m.retrieveErr != nil {
		return infisical.Secret{}, m.retrieveErr
	}

	return m.retrieveResp, nil
}

func (m *mockInfisicalSecretsClient) Create(options infisical.CreateSecretOptions) (infisical.Secret, error) {
	m.createReq = options
	if m.createErr != nil {
		return infisical.Secret{}, m.createErr
	}

	return infisical.Secret{}, nil
}

func (m *mockInfisicalSecretsClient) Update(options infisical.UpdateSecretOptions) (infisical.Secret, error) {
	m.updateReq = options
	if m.updateErr != nil {
		return infisical.Secret{}, m.updateErr
	}

	return infisical.Secret{}, nil
}

func (m *mockInfisicalSecretsClient) Delete(options infisical.DeleteSecretOptions) (infisical.Secret, error) {
	m.deleteReq = options
	if m.deleteErr != nil {
		return infisical.Secret{}, m.deleteErr
	}

	return infisical.Secret{}, nil
}

func TestInfisicalSecretStore_CRUDMapping(t *testing.T) {
	const (
		projectID   = "project-123"
		environment = "prod"
		secretID    = "node-abc"
		secretValue = "new-password"
	)

	tests := []struct {
		name   string
		run    func(store *InfisicalSecretStore) (string, error)
		assert func(t *testing.T, client *mockInfisicalSecretsClient, got string)
	}{
		{
			name: "read maps id project and environment",
			run: func(store *InfisicalSecretStore) (string, error) {
				return store.Read(secretID)
			},
			assert: func(t *testing.T, client *mockInfisicalSecretsClient, got string) {
				t.Helper()
				if client.retrieveReq.SecretKey != secretID {
					t.Fatalf("expected SecretKey %q, got %q", secretID, client.retrieveReq.SecretKey)
				}
				if client.retrieveReq.ProjectID != projectID {
					t.Fatalf("expected ProjectID %q, got %q", projectID, client.retrieveReq.ProjectID)
				}
				if client.retrieveReq.Environment != environment {
					t.Fatalf("expected Environment %q, got %q", environment, client.retrieveReq.Environment)
				}
				if got != secretValue {
					t.Fatalf("expected returned secret value %q, got %q", secretValue, got)
				}
			},
		},
		{
			name: "write maps id data project and environment",
			run: func(store *InfisicalSecretStore) (string, error) {
				return "", store.Write(secretID, secretValue)
			},
			assert: func(t *testing.T, client *mockInfisicalSecretsClient, got string) {
				t.Helper()
				if client.createReq.SecretKey != secretID {
					t.Fatalf("expected SecretKey %q, got %q", secretID, client.createReq.SecretKey)
				}
				if client.createReq.ProjectID != projectID {
					t.Fatalf("expected ProjectID %q, got %q", projectID, client.createReq.ProjectID)
				}
				if client.createReq.Environment != environment {
					t.Fatalf("expected Environment %q, got %q", environment, client.createReq.Environment)
				}
				if client.createReq.SecretValue != secretValue {
					t.Fatalf("expected SecretValue %q, got %q", secretValue, client.createReq.SecretValue)
				}
			},
		},
		{
			name: "update maps id data project and environment",
			run: func(store *InfisicalSecretStore) (string, error) {
				return "", store.Update(secretID, secretValue)
			},
			assert: func(t *testing.T, client *mockInfisicalSecretsClient, got string) {
				t.Helper()
				if client.updateReq.SecretKey != secretID {
					t.Fatalf("expected SecretKey %q, got %q", secretID, client.updateReq.SecretKey)
				}
				if client.updateReq.ProjectID != projectID {
					t.Fatalf("expected ProjectID %q, got %q", projectID, client.updateReq.ProjectID)
				}
				if client.updateReq.Environment != environment {
					t.Fatalf("expected Environment %q, got %q", environment, client.updateReq.Environment)
				}
				if client.updateReq.NewSecretValue != secretValue {
					t.Fatalf("expected NewSecretValue %q, got %q", secretValue, client.updateReq.NewSecretValue)
				}
			},
		},
		{
			name: "delete maps id project and environment",
			run: func(store *InfisicalSecretStore) (string, error) {
				return "", store.Delete(secretID)
			},
			assert: func(t *testing.T, client *mockInfisicalSecretsClient, got string) {
				t.Helper()
				if client.deleteReq.SecretKey != secretID {
					t.Fatalf("expected SecretKey %q, got %q", secretID, client.deleteReq.SecretKey)
				}
				if client.deleteReq.ProjectID != projectID {
					t.Fatalf("expected ProjectID %q, got %q", projectID, client.deleteReq.ProjectID)
				}
				if client.deleteReq.Environment != environment {
					t.Fatalf("expected Environment %q, got %q", environment, client.deleteReq.Environment)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &mockInfisicalSecretsClient{
				retrieveResp: infisical.Secret{SecretValue: secretValue},
			}
			store := &InfisicalSecretStore{
				secrets:     client,
				projectID:   projectID,
				environment: environment,
			}

			got, err := tc.run(store)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			tc.assert(t, client, got)
		})
	}
}

func TestNewInfisicalSecretStore_MissingEnv(t *testing.T) {
	vars := []string{
		envInfisicalClientID,
		envInfisicalClientSecret,
		envInfisicalProjectID,
		envInfisicalEnvironment,
	}

	original := make(map[string]string, len(vars))
	for _, key := range vars {
		original[key] = osLookupEnvOrEmpty(key)
		_ = osUnsetenv(key)
	}
	t.Cleanup(func() {
		for _, key := range vars {
			if original[key] == "" {
				_ = osUnsetenv(key)
				continue
			}
			_ = osSetenv(key, original[key])
		}
	})

	_, err := NewInfisicalSecretStore()
	if err == nil {
		t.Fatalf("expected error when required env vars are missing")
	}

	for _, key := range vars {
		if !strings.Contains(err.Error(), key) {
			t.Fatalf("expected error to mention %q, got %v", key, err)
		}
	}
}

func TestInfisicalSecretStore_PropagatesSDKErrors(t *testing.T) {
	store := &InfisicalSecretStore{
		secrets: &mockInfisicalSecretsClient{
			retrieveErr: fmt.Errorf("retrieve failed"),
			createErr:   fmt.Errorf("create failed"),
			updateErr:   fmt.Errorf("update failed"),
			deleteErr:   fmt.Errorf("delete failed"),
		},
		projectID:   "p",
		environment: "e",
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "read", run: func() error { _, err := store.Read("id"); return err }},
		{name: "write", run: func() error { return store.Write("id", "v") }},
		{name: "update", run: func() error { return store.Update("id", "v") }},
		{name: "delete", run: func() error { return store.Delete("id") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

// Wrappers keep env mutation in tests local and explicit.
func osLookupEnvOrEmpty(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return ""
	}
	return value
}

func osUnsetenv(key string) error {
	return os.Unsetenv(key)
}

func osSetenv(key string, value string) error {
	return os.Setenv(key, value)
}
