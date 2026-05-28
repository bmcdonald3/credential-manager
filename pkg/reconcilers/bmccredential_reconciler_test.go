package reconcilers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/openchami/fabrica/pkg/reconcile"
	"github.com/user/credential-manager/apis/example.fabrica.dev/v1"
)

type fakeSecretStore struct {
	secrets map[string]string
	errors  map[string]error
}

func (f *fakeSecretStore) LookupSecret(ctx context.Context, id string) (string, error) {
	_ = ctx
	if err, ok := f.errors[id]; ok {
		return "", err
	}
	v, ok := f.secrets[id]
	if !ok {
		return "", errors.New("secret not found")
	}
	return v, nil
}

type fakeHTTPClient struct {
	statusCode int
	err        error
	lastReq    *http.Request
	body       string
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.lastReq = req
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		f.body = string(bodyBytes)
	}

	if f.err != nil {
		return nil, f.err
	}

	if f.statusCode == 0 {
		f.statusCode = http.StatusNoContent
	}

	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func TestValidateBmcCredentialSecretIDs(t *testing.T) {
	tests := []struct {
		name    string
		spec    v1.BmcCredentialSpec
		wantErr string
	}{
		{
			name: "valid secret IDs",
			spec: v1.BmcCredentialSpec{
				CurrentPasswordSecretID: "secret-current-1",
				NewPasswordSecretID:     "secret-new-1",
			},
		},
		{
			name: "missing current password secret ID",
			spec: v1.BmcCredentialSpec{
				CurrentPasswordSecretID: "",
				NewPasswordSecretID:     "secret-new-1",
			},
			wantErr: "currentPasswordSecretID must not be empty",
		},
		{
			name: "missing new password secret ID",
			spec: v1.BmcCredentialSpec{
				CurrentPasswordSecretID: "secret-current-1",
				NewPasswordSecretID:     "",
			},
			wantErr: "newPasswordSecretID must not be empty",
		},
		{
			name: "whitespace-only secret IDs",
			spec: v1.BmcCredentialSpec{
				CurrentPasswordSecretID: "   ",
				NewPasswordSecretID:     "secret-new-1",
			},
			wantErr: "currentPasswordSecretID must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBmcCredentialSecretIDs(tt.spec)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestReconcileBmcCredentialValidationStatus(t *testing.T) {
	tests := []struct {
		name        string
		spec        v1.BmcCredentialSpec
		secretStore SecretResolver
		httpClient  HTTPDoer
		wantState   string
		wantReason  string
		assert      func(t *testing.T, client *fakeHTTPClient)
	}{
		{
			name: "success when secrets resolve and redfish returns 204",
			spec: v1.BmcCredentialSpec{
				TargetAddress:           "bmc-1.local",
				CurrentUsername:         "admin",
				CurrentPasswordSecretID: "current-secret",
				TargetAccount:           "admin",
				NewPasswordSecretID:     "new-secret",
			},
			secretStore: &fakeSecretStore{secrets: map[string]string{
				"current-secret": "old-password",
				"new-secret":     "new-password",
			}},
			httpClient: &fakeHTTPClient{statusCode: http.StatusNoContent},
			wantState:  "Success",
			assert: func(t *testing.T, c *fakeHTTPClient) {
				t.Helper()
				if c.lastReq == nil {
					t.Fatal("expected HTTP request to be issued")
				}
				if got := c.lastReq.URL.String(); got != "https://bmc-1.local/redfish/v1/AccountService/Accounts/admin" {
					t.Fatalf("unexpected request URL: %s", got)
				}
				user, pass, ok := c.lastReq.BasicAuth()
				if !ok || user != "admin" || pass != "old-password" {
					t.Fatalf("unexpected basic auth credentials: user=%q pass=%q", user, pass)
				}
				if c.body != `{"Password":"new-password"}` {
					t.Fatalf("unexpected request body: %s", c.body)
				}
			},
		},
		{
			name: "failed when current secret ID is missing",
			spec: v1.BmcCredentialSpec{
				CurrentPasswordSecretID: "",
				NewPasswordSecretID:     "new-secret",
			},
			wantState:  "Failed",
			wantReason: "currentPasswordSecretID must not be empty",
		},
		{
			name: "failed when secret resolution fails",
			spec: v1.BmcCredentialSpec{
				TargetAddress:           "bmc-1.local",
				CurrentUsername:         "admin",
				CurrentPasswordSecretID: "current-secret",
				TargetAccount:           "admin",
				NewPasswordSecretID:     "new-secret",
			},
			secretStore: &fakeSecretStore{
				secrets: map[string]string{
					"new-secret": "new-password",
				},
				errors: map[string]error{
					"current-secret": errors.New("decrypt failed"),
				},
			},
			wantState:  "Failed",
			wantReason: "failed to resolve currentPasswordSecretID",
		},
		{
			name: "failed when redfish rejects credentials",
			spec: v1.BmcCredentialSpec{
				TargetAddress:           "bmc-1.local",
				CurrentUsername:         "admin",
				CurrentPasswordSecretID: "current-secret",
				TargetAccount:           "admin",
				NewPasswordSecretID:     "new-secret",
			},
			secretStore: &fakeSecretStore{secrets: map[string]string{
				"current-secret": "old-password",
				"new-secret":     "new-password",
			}},
			httpClient: &fakeHTTPClient{statusCode: http.StatusUnauthorized},
			wantState:  "Failed",
			wantReason: "redfish patch failed: HTTP 401",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &BmcCredentialReconciler{
				BaseReconciler: reconcile.BaseReconciler{Logger: reconcile.NewDefaultLogger()},
				SecretStore:    tt.secretStore,
				HTTPClient:     tt.httpClient,
			}

			res := &v1.BmcCredential{Spec: tt.spec}
			if err := reconciler.reconcileBmcCredential(context.Background(), res); err != nil {
				t.Fatalf("expected no reconcile error, got %v", err)
			}

			if res.Status.State != tt.wantState {
				t.Fatalf("expected state %q, got %q", tt.wantState, res.Status.State)
			}

			if tt.wantReason == "" && res.Status.ValidationFailureReason != "" {
				t.Fatalf("expected empty reason, got %q", res.Status.ValidationFailureReason)
			}

			if tt.wantReason != "" && !strings.Contains(res.Status.ValidationFailureReason, tt.wantReason) {
				t.Fatalf("expected reason %q, got %q", tt.wantReason, res.Status.ValidationFailureReason)
			}

			if res.Status.LastUpdatedAtUTC.IsZero() {
				t.Fatal("expected lastUpdatedAtUTC to be set")
			}

			if tt.assert != nil {
				if client, ok := tt.httpClient.(*fakeHTTPClient); ok {
					tt.assert(t, client)
				}
			}
		})
	}
}
