package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"

	infisical "github.com/infisical/go-sdk"
)

const (
	envInfisicalClientID     = "INFISICAL_CLIENT_ID"
	envInfisicalClientSecret = "INFISICAL_CLIENT_SECRET"
	envInfisicalProjectID    = "INFISICAL_PROJECT_ID"
	envInfisicalEnvironment  = "INFISICAL_ENVIRONMENT"
)

// infisicalSecretsClient keeps the adapter focused on the subset of SDK calls it needs.
type infisicalSecretsClient interface {
	Retrieve(options infisical.RetrieveSecretOptions) (infisical.Secret, error)
	Create(options infisical.CreateSecretOptions) (infisical.Secret, error)
	Update(options infisical.UpdateSecretOptions) (infisical.Secret, error)
	Delete(options infisical.DeleteSecretOptions) (infisical.Secret, error)
}

// InfisicalSecretStore is a SecretStore backed by Infisical secrets.
type InfisicalSecretStore struct {
	secrets     infisicalSecretsClient
	projectID   string
	environment string
}

// NewInfisicalSecretStore builds an Infisical secret store authenticated via Universal Auth.
func NewInfisicalSecretStore() (*InfisicalSecretStore, error) {
	clientID := strings.TrimSpace(os.Getenv(envInfisicalClientID))
	clientSecret := strings.TrimSpace(os.Getenv(envInfisicalClientSecret))
	projectID := strings.TrimSpace(os.Getenv(envInfisicalProjectID))
	environment := strings.TrimSpace(os.Getenv(envInfisicalEnvironment))

	missing := make([]string, 0, 4)
	if clientID == "" {
		missing = append(missing, envInfisicalClientID)
	}
	if clientSecret == "" {
		missing = append(missing, envInfisicalClientSecret)
	}
	if projectID == "" {
		missing = append(missing, envInfisicalProjectID)
	}
	if environment == "" {
		missing = append(missing, envInfisicalEnvironment)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required Infisical environment variables: %s", strings.Join(missing, ", "))
	}

	client := infisical.NewInfisicalClient(context.Background(), infisical.Config{})
	if _, err := client.Auth().UniversalAuthLogin(clientID, clientSecret); err != nil {
		return nil, fmt.Errorf("failed to authenticate with Infisical Universal Auth: %w", err)
	}

	return &InfisicalSecretStore{
		secrets:     client.Secrets(),
		projectID:   projectID,
		environment: environment,
	}, nil
}

func (s *InfisicalSecretStore) Read(id string) (string, error) {
	secret, err := s.secrets.Retrieve(infisical.RetrieveSecretOptions{
		SecretKey:   id,
		ProjectID:   s.projectID,
		Environment: s.environment,
	})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve secret %q from Infisical: %w", id, err)
	}

	return secret.SecretValue, nil
}

func (s *InfisicalSecretStore) Write(id string, data string) error {
	_, err := s.secrets.Create(infisical.CreateSecretOptions{
		SecretKey:   id,
		ProjectID:   s.projectID,
		Environment: s.environment,
		SecretValue: data,
	})
	if err != nil {
		return fmt.Errorf("failed to create secret %q in Infisical: %w", id, err)
	}

	return nil
}

func (s *InfisicalSecretStore) Update(id string, data string) error {
	_, err := s.secrets.Update(infisical.UpdateSecretOptions{
		SecretKey:      id,
		ProjectID:      s.projectID,
		Environment:    s.environment,
		NewSecretValue: data,
	})
	if err != nil {
		return fmt.Errorf("failed to update secret %q in Infisical: %w", id, err)
	}

	return nil
}

func (s *InfisicalSecretStore) Delete(id string) error {
	_, err := s.secrets.Delete(infisical.DeleteSecretOptions{
		SecretKey:   id,
		ProjectID:   s.projectID,
		Environment: s.environment,
	})
	if err != nil {
		return fmt.Errorf("failed to delete secret %q in Infisical: %w", id, err)
	}

	return nil
}
