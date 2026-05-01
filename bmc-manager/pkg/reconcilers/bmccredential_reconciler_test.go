package reconcilers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/openchami/fabrica/pkg/reconcile"
	v1 "github.com/user/bmc-manager/apis/example.fabrica.dev/v1"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestReconcileBmcCredential(t *testing.T) {
	fixedTime := time.Date(2026, time.May, 1, 10, 0, 0, 0, time.UTC)
	originalNowUTC := nowUTC
	nowUTC = func() time.Time { return fixedTime }
	t.Cleanup(func() {
		nowUTC = originalNowUTC
	})

	tests := []struct {
		name              string
		transport         roundTripperFunc
		expectVerified    bool
		expectFailurePart string
	}{
		{
			name: "marks verified true on 200",
			transport: func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://bmc-01.example.com/redfish/v1/" {
					t.Fatalf("unexpected URL: %s", req.URL.String())
				}
				username, password, ok := req.BasicAuth()
				if !ok {
					t.Fatalf("expected basic auth header")
				}
				if username != "admin" || password != "secret" {
					t.Fatalf("unexpected credentials: %s/%s", username, password)
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("{}")),
				}, nil
			},
			expectVerified: true,
		},
		{
			name: "marks verified false on non-200",
			transport: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(strings.NewReader("{}")),
				}, nil
			},
			expectVerified:    false,
			expectFailurePart: "status 401",
		},
		{
			name: "marks verified false on timeout-like transport error",
			transport: func(req *http.Request) (*http.Response, error) {
				return nil, context.DeadlineExceeded
			},
			expectVerified:    false,
			expectFailurePart: "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalClient := defaultBMCVerifyClient
			defaultBMCVerifyClient = &http.Client{Transport: tt.transport}
			t.Cleanup(func() {
				defaultBMCVerifyClient = originalClient
			})

			r := &BmcCredentialReconciler{
				BaseReconciler: reconcile.BaseReconciler{Logger: reconcile.NewDefaultLogger()},
			}
			res := &v1.BmcCredential{
				Spec: v1.BmcCredentialSpec{
					Address:  "bmc-01.example.com",
					Username: "admin",
					Password: "secret",
				},
			}

			if err := r.reconcileBmcCredential(context.Background(), res); err != nil {
				t.Fatalf("reconcileBmcCredential returned error: %v", err)
			}

			if !res.Status.LastCheckedAt.Equal(fixedTime) {
				t.Fatalf("expected LastCheckedAt %v, got %v", fixedTime, res.Status.LastCheckedAt)
			}

			if res.Status.Verified != tt.expectVerified {
				t.Fatalf("expected verified %v, got %v", tt.expectVerified, res.Status.Verified)
			}

			if tt.expectFailurePart == "" {
				if res.Status.FailureReason != "" {
					t.Fatalf("expected empty failure reason, got %q", res.Status.FailureReason)
				}
				if !res.Status.Ready {
					t.Fatalf("expected ready true when verification succeeds")
				}
				return
			}

			if !strings.Contains(res.Status.FailureReason, tt.expectFailurePart) {
				t.Fatalf("expected failure reason to contain %q, got %q", tt.expectFailurePart, res.Status.FailureReason)
			}
			if res.Status.Ready {
				t.Fatalf("expected ready false when verification fails")
			}
		})
	}
}
