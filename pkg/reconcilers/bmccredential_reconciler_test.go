package reconcilers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/openchami/fabrica/pkg/reconcile"
	"github.com/user/test/apis/example.fabrica.dev/v1"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRotateBmcPassword(t *testing.T) {
	t.Parallel()

	spec := v1.BmcCredentialSpec{
		TargetAddress:   "10.1.1.1",
		CurrentUsername: "admin",
		CurrentPassword: "old-pass",
		TargetAccount:   "acct 2",
		NewPassword:     "new-pass",
	}

	tests := []struct {
		name        string
		transport   roundTripFunc
		expectErr   bool
		errContains []string
	}{
		{
			name: "patch succeeds with 204",
			transport: func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodPatch {
					t.Fatalf("expected PATCH, got %s", req.Method)
				}
				expectedURL := "https://10.1.1.1/redfish/v1/AccountService/Accounts/acct%202"
				if req.URL.String() != expectedURL {
					t.Fatalf("expected URL %s, got %s", expectedURL, req.URL.String())
				}

				user, pass, ok := req.BasicAuth()
				if !ok || user != "admin" || pass != "old-pass" {
					t.Fatalf("unexpected basic auth credentials user=%q pass=%q ok=%v", user, pass, ok)
				}

				body, err := io.ReadAll(req.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}
				if strings.TrimSpace(string(body)) != `{"Password":"new-pass"}` {
					t.Fatalf("unexpected payload: %s", strings.TrimSpace(string(body)))
				}

				return &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			},
		},
		{
			name: "unauthorized response",
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(strings.NewReader("invalid credentials")),
				}, nil
			},
			expectErr:   true,
			errContains: []string{"401", "invalid credentials"},
		},
		{
			name: "network error",
			transport: func(req *http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			},
			expectErr:   true,
			errContains: []string{"request failed", "context deadline exceeded"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &http.Client{Transport: tt.transport}
			err := rotateBmcPassword(context.Background(), client, spec)
			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				for _, part := range tt.errContains {
					if !strings.Contains(err.Error(), part) {
						t.Fatalf("expected error to contain %q, got %q", part, err.Error())
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

func TestReconcileBmcCredentialStatusUpdates(t *testing.T) {
	tests := []struct {
		name             string
		transport        roundTripFunc
		expectSucceeded  bool
		expectReasonLike string
	}{
		{
			name: "sets success status on 200",
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
			},
			expectSucceeded: true,
		},
		{
			name: "sets failure status on unauthorized",
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusUnauthorized, Body: io.NopCloser(strings.NewReader("unauthorized"))}, nil
			},
			expectSucceeded:  false,
			expectReasonLike: "401",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			oldFactory := bmcHTTPClientFactory
			t.Cleanup(func() { bmcHTTPClientFactory = oldFactory })

			bmcHTTPClientFactory = func() *http.Client {
				return &http.Client{Transport: tt.transport}
			}

			reconciler := &BmcCredentialReconciler{
				BaseReconciler: reconcile.BaseReconciler{Logger: reconcile.NewDefaultLogger()},
			}

			res := &v1.BmcCredential{
				Metadata: v1.BmcCredential{}.Metadata,
				Spec: v1.BmcCredentialSpec{
					TargetAddress:   "127.0.0.1",
					CurrentUsername: "admin",
					CurrentPassword: "old-pass",
					TargetAccount:   "2",
					NewPassword:     "new-pass",
				},
			}

			if err := reconciler.reconcileBmcCredential(context.Background(), res); err != nil {
				t.Fatalf("expected nil reconcile error, got %v", err)
			}

			if res.Status.LastRotationAttempt == nil {
				t.Fatalf("expected LastRotationAttempt to be set")
			}
			if res.Status.LastRotationAttempt.Location() != time.UTC {
				t.Fatalf("expected UTC timestamp, got location %v", res.Status.LastRotationAttempt.Location())
			}

			if res.Status.RotationSucceeded != tt.expectSucceeded {
				t.Fatalf("expected RotationSucceeded=%v, got %v", tt.expectSucceeded, res.Status.RotationSucceeded)
			}

			if tt.expectSucceeded {
				if res.Status.FailureReason != "" {
					t.Fatalf("expected empty failure reason, got %q", res.Status.FailureReason)
				}
			} else if !strings.Contains(res.Status.FailureReason, tt.expectReasonLike) {
				t.Fatalf("expected failure reason to contain %q, got %q", tt.expectReasonLike, res.Status.FailureReason)
			}
		})
	}
}
