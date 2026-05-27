package reconcilers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/user/credential-manager/apis/example.fabrica.dev/v1"
	"github.com/user/credential-manager/pkg/secrets"
)

func TestReconcileBmcCredential(t *testing.T) {
	const (
		nodeID      = "node-123"
		username    = "admin"
		oldPassword = "old-password"
	)

	tests := []struct {
		name              string
		serverHandler     func(t *testing.T, w http.ResponseWriter, r *http.Request)
		contextTimeout    time.Duration
		authUsername      string
		wantErr           bool
		errContains       string
		wantSuccessStatus bool
	}{
		{
			name: "success with 204 updates secret store",
			serverHandler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch {
					t.Fatalf("expected method PATCH, got %s", r.Method)
				}
				if r.URL.Path != "/redfish/v1/AccountService/Accounts/admin" {
					t.Fatalf("unexpected path: %s", r.URL.Path)
				}
				user, pass, ok := r.BasicAuth()
				if !ok {
					t.Fatalf("expected basic auth")
				}
				if user != username || pass != oldPassword {
					t.Fatalf("unexpected credentials user=%q pass=%q", user, pass)
				}
				w.WriteHeader(http.StatusNoContent)
			},
			authUsername:      username,
			wantErr:           false,
			wantSuccessStatus: true,
		},
		{
			name: "unauthorized response sets failure reason",
			serverHandler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte("invalid credentials"))
			},
			authUsername:      username,
			wantErr:           true,
			errContains:       "401 Unauthorized",
			wantSuccessStatus: false,
		},
		{
			name: "timeout sets failure reason",
			serverHandler: func(t *testing.T, w http.ResponseWriter, r *http.Request) {
				time.Sleep(100 * time.Millisecond)
				w.WriteHeader(http.StatusNoContent)
			},
			authUsername:      username,
			contextTimeout:    10 * time.Millisecond,
			wantErr:           true,
			errContains:       "timeout",
			wantSuccessStatus: false,
		},
		{
			name:              "missing auth username fails validation",
			authUsername:      "   ",
			wantErr:           true,
			errContains:       "authUsername",
			wantSuccessStatus: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := secrets.NewLocalSecretStore()
			if err := store.Update(nodeID, oldPassword); err != nil {
				if err := store.Write(nodeID, oldPassword); err != nil {
					t.Fatalf("failed to seed secret store: %v", err)
				}
			}

			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tc.serverHandler(t, w, r)
			}))
			defer server.Close()

			reconciler := NewBmcCredentialReconciler(nil, nil, store)
			resource := &v1.BmcCredential{
				Spec: v1.BmcCredentialSpec{
					Address:        strings.TrimPrefix(server.URL, "https://"),
					AuthUsername:   tc.authUsername,
					TargetAccount:  username,
					NodeIdentifier: nodeID,
				},
			}

			ctx := context.Background()
			var cancel context.CancelFunc
			if tc.contextTimeout > 0 {
				ctx, cancel = context.WithTimeout(ctx, tc.contextTimeout)
				defer cancel()
			}

			err := reconciler.reconcileBmcCredential(ctx, resource)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("did not expect error, got: %v", err)
			}
			if tc.errContains != "" && (err == nil || !strings.Contains(err.Error(), tc.errContains)) {
				t.Fatalf("expected error containing %q, got: %v", tc.errContains, err)
			}

			if resource.Status.LastRotationUTC == "" {
				t.Fatalf("expected last rotation timestamp to be set")
			}
			if resource.Status.RotationSucceeded != tc.wantSuccessStatus {
				t.Fatalf("expected success status %v, got %v", tc.wantSuccessStatus, resource.Status.RotationSucceeded)
			}

			stored, readErr := store.Read(nodeID)
			if readErr != nil {
				t.Fatalf("failed reading stored secret: %v", readErr)
			}

			if tc.wantSuccessStatus {
				if stored == oldPassword {
					t.Fatalf("expected rotated password to differ from original")
				}
				if resource.Status.FailureReason != "" {
					t.Fatalf("expected empty failure reason, got %q", resource.Status.FailureReason)
				}
			} else {
				if stored != oldPassword {
					t.Fatalf("expected stored secret to remain unchanged on failure")
				}
				if resource.Status.FailureReason == "" {
					t.Fatalf("expected failure reason to be populated")
				}
			}
		})
	}
}
