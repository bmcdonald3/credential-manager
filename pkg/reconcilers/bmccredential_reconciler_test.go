package reconcilers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	v1 "github.com/openchami/credential-manager/apis/credentials.openchami.org/v1"
	"github.com/openchami/credential-manager/pkg/secrets"
)

type stubSecretStore struct {
	secret string
	err    error
}

func (s stubSecretStore) GetSecretByID(secretID string) (string, error) {
	if s.err != nil {
		return "", s.err
	}

	return s.secret, nil
}

func (s stubSecretStore) StoreSecretByID(secretID, secret string) error {
	return nil
}

func (s stubSecretStore) ListSecrets() (map[string]string, error) {
	return map[string]string{}, nil
}

func (s stubSecretStore) RemoveSecretByID(secretID string) error {
	return nil
}

func TestShouldRotate(t *testing.T) {
	now := time.Now().UTC()

	testCases := []struct {
		name string
		res  *v1.BmcCredential
		want bool
	}{
		{
			name: "initial creation triggers rotation",
			res: &v1.BmcCredential{
				Spec: v1.BmcCredentialSpec{RotationTrigger: "abc"},
				Status: v1.BmcCredentialStatus{
					LastRotationAttempt: nil,
				},
			},
			want: true,
		},
		{
			name: "unchanged trigger does not rotate",
			res: &v1.BmcCredential{
				Spec: v1.BmcCredentialSpec{RotationTrigger: "abc"},
				Status: v1.BmcCredentialStatus{
					LastRotationAttempt:  &now,
					ObservedTriggerValue: "abc",
				},
			},
			want: false,
		},
		{
			name: "changed trigger rotates",
			res: &v1.BmcCredential{
				Spec: v1.BmcCredentialSpec{RotationTrigger: "next"},
				Status: v1.BmcCredentialStatus{
					LastRotationAttempt:  &now,
					ObservedTriggerValue: "prev",
				},
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := shouldRotate(tc.res)
			if got != tc.want {
				t.Fatalf("shouldRotate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldRotateStopsAfterTerminalFailureForSameTrigger(t *testing.T) {
	res := &v1.BmcCredential{
		Spec: v1.BmcCredentialSpec{RotationTrigger: "same"},
		Status: v1.BmcCredentialStatus{
			LastRotationAttempt:  ptrTime(time.Now().UTC()),
			ObservedTriggerValue: "same",
			RotationSucceeded:    false,
			FailureReason:        "failed to load secret",
		},
	}

	if shouldRotate(res) {
		t.Fatal("expected unchanged trigger to remain idle after terminal failure")
	}
}

func TestRotateRedfishPassword(t *testing.T) {
	previousFactory := newSecretStore
	previousClientFactory := buildRedfishHTTPClient
	previousMasterKey := os.Getenv(masterKeyEnvVar)
	t.Cleanup(func() {
		newSecretStore = previousFactory
		buildRedfishHTTPClient = previousClientFactory
		if previousMasterKey == "" {
			_ = os.Unsetenv(masterKeyEnvVar)
			return
		}
		_ = os.Setenv(masterKeyEnvVar, previousMasterKey)
	})

	if err := os.Setenv(masterKeyEnvVar, strings.Repeat("a", 64)); err != nil {
		t.Fatalf("failed to set %s: %v", masterKeyEnvVar, err)
	}

	testCases := []struct {
		name           string
		secret         string
		secretErr      error
		statusCode     int
		responseBody   string
		wantSuccess    bool
		wantFailureSub string
		wantTerminal   bool
	}{
		{
			name:         "http 204 is success",
			secret:       `{"currentUsername":"admin","currentPassword":"oldpass","newPassword":"newpass"}`,
			statusCode:   http.StatusNoContent,
			responseBody: "",
			wantSuccess:  true,
		},
		{
			name:           "http 401 is terminal failure",
			secret:         `{"currentUsername":"admin","currentPassword":"oldpass","newPassword":"newpass"}`,
			statusCode:     http.StatusUnauthorized,
			responseBody:   "invalid credentials",
			wantSuccess:    false,
			wantFailureSub: "received HTTP 401",
			wantTerminal:   true,
		},
		{
			name:           "http 503 is transient failure",
			secret:         `{"currentUsername":"admin","currentPassword":"oldpass","newPassword":"newpass"}`,
			statusCode:     http.StatusServiceUnavailable,
			responseBody:   "busy",
			wantSuccess:    false,
			wantFailureSub: "received HTTP 503",
			wantTerminal:   false,
		},
		{
			name:           "missing secret is terminal failure",
			secretErr:      errors.New("no secret found"),
			wantSuccess:    false,
			wantFailureSub: "failed to load secret",
			wantTerminal:   true,
		},
		{
			name:           "invalid secret payload is terminal failure",
			secret:         `{"username":"admin"}`,
			wantSuccess:    false,
			wantFailureSub: "invalid secret payload JSON",
			wantTerminal:   true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			newSecretStore = func(masterKeyHex, filename string, create bool) (secrets.SecretStore, error) {
				return stubSecretStore{secret: tc.secret, err: tc.secretErr}, nil
			}

			targetHost := "bmc.example.com"
			buildRedfishHTTPClient = func() *http.Client {
				return &http.Client{Timeout: 15 * time.Second}
			}

			if tc.secretErr == nil && tc.secret != "" {
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodPatch {
						t.Fatalf("expected PATCH method, got %s", r.Method)
					}

					if r.URL.Path != "/redfish/v1/AccountService/Accounts/root" {
						t.Fatalf("unexpected request path: %s", r.URL.Path)
					}

					username, password, ok := r.BasicAuth()
					if !ok {
						t.Fatal("expected basic auth credentials")
					}
					if username != "admin" || password != "oldpass" {
						t.Fatalf("unexpected basic auth credentials: %s/%s", username, password)
					}

					payload, err := io.ReadAll(r.Body)
					if err != nil {
						t.Fatalf("failed to read request body: %v", err)
					}

					var body map[string]string
					if err := json.Unmarshal(payload, &body); err != nil {
						t.Fatalf("failed to unmarshal request body: %v", err)
					}
					if body["Password"] != "newpass" {
						t.Fatalf("unexpected password payload: %s", body["Password"])
					}

					w.WriteHeader(tc.statusCode)
					if tc.responseBody != "" {
						_, _ = w.Write([]byte(tc.responseBody))
					}
				}))
				defer server.Close()

				targetURL, err := url.Parse(server.URL)
				if err != nil {
					t.Fatalf("failed to parse server URL: %v", err)
				}

				targetHost = targetURL.Host
				buildRedfishHTTPClient = server.Client
			}

			spec := v1.BmcCredentialSpec{
				TargetAddress: targetHost,
				TargetAccount: "root",
				SecretID:      "secret-1",
			}

			succeeded, failureReason, err := rotateRedfishPassword(context.Background(), spec)
			if succeeded != tc.wantSuccess {
				t.Fatalf("rotateRedfishPassword() success = %v, want %v", succeeded, tc.wantSuccess)
			}

			if tc.wantSuccess {
				if err != nil {
					t.Fatalf("expected nil error for success case, got %v", err)
				}
				if failureReason != "" {
					t.Fatalf("expected empty failure reason for success case, got %q", failureReason)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error for failure case, got nil")
			}
			if !strings.Contains(failureReason, tc.wantFailureSub) {
				t.Fatalf("failure reason %q does not contain %q", failureReason, tc.wantFailureSub)
			}
			if isTerminalError(err) != tc.wantTerminal {
				t.Fatalf("isTerminalError(%v) = %v, want %v", err, isTerminalError(err), tc.wantTerminal)
			}
		})
	}
}

func TestRotateRedfishPasswordMissingMasterKeyIsTerminal(t *testing.T) {
	previousMasterKey := os.Getenv(masterKeyEnvVar)
	t.Cleanup(func() {
		if previousMasterKey == "" {
			_ = os.Unsetenv(masterKeyEnvVar)
			return
		}
		_ = os.Setenv(masterKeyEnvVar, previousMasterKey)
	})

	if err := os.Unsetenv(masterKeyEnvVar); err != nil {
		t.Fatalf("failed to clear %s: %v", masterKeyEnvVar, err)
	}

	succeeded, failureReason, err := rotateRedfishPassword(context.Background(), v1.BmcCredentialSpec{
		TargetAddress: "bmc.example.com",
		TargetAccount: "root",
		SecretID:      "secret-1",
	})

	if succeeded {
		t.Fatal("expected rotation to fail")
	}
	if !strings.Contains(failureReason, "MASTER_KEY") {
		t.Fatalf("unexpected failure reason: %q", failureReason)
	}
	if !isTerminalError(err) {
		t.Fatalf("expected terminal error, got %v", err)
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
