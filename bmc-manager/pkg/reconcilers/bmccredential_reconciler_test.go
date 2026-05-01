package reconcilers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/user/bmc-manager/apis/example.fabrica.dev/v1"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestReconcileBmcCredential_Verification(t *testing.T) {
	originalFactory := newHTTPClient
	t.Cleanup(func() {
		newHTTPClient = originalFactory
	})

	tests := []struct {
		name                 string
		statusCode           int
		roundTripErr         error
		expectVerified       bool
		expectFailureSubstr  string
	}{
		{
			name:               "marks verified on HTTP 200",
			statusCode:         http.StatusOK,
			expectVerified:     true,
		},
		{
			name:               "marks unverified on non-200",
			statusCode:         http.StatusUnauthorized,
			expectVerified:     false,
			expectFailureSubstr: "HTTP 401",
		},
		{
			name:               "marks unverified on transport error",
			roundTripErr:       errors.New("dial tcp timeout"),
			expectVerified:     false,
			expectFailureSubstr: "dial tcp timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newHTTPClient = func() *http.Client {
				return &http.Client{
					Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
						if req.Method != http.MethodGet {
							t.Fatalf("expected GET request, got %s", req.Method)
						}

						if req.URL.String() != "https://bmc.example.local/redfish/v1/" {
							t.Fatalf("unexpected request URL: %s", req.URL.String())
						}

						user, pass, ok := req.BasicAuth()
						if !ok {
							t.Fatalf("expected basic auth to be set")
						}
						if user != "admin" || pass != "password123" {
							t.Fatalf("unexpected basic auth credentials: %s/%s", user, pass)
						}

						if tt.roundTripErr != nil {
							return nil, tt.roundTripErr
						}

						return &http.Response{
							StatusCode: tt.statusCode,
							Status:     fmt.Sprintf("%d %s", tt.statusCode, http.StatusText(tt.statusCode)),
							Body:       http.NoBody,
							Header:     make(http.Header),
						}, nil
					}),
				}
			}

			resource := &v1.BmcCredential{
				Spec: v1.BmcCredentialSpec{
					Address:  "bmc.example.local",
					Username: "admin",
					Password: "password123",
				},
			}

			before := time.Now().UTC()
			err := (&BmcCredentialReconciler{}).reconcileBmcCredential(context.Background(), resource)
			after := time.Now().UTC()

			if err != nil {
				t.Fatalf("unexpected reconcile error: %v", err)
			}

			if resource.Status.Verified != tt.expectVerified {
				t.Fatalf("expected verified=%v, got %v", tt.expectVerified, resource.Status.Verified)
			}

			if resource.Status.LastCheckedAt.IsZero() {
				t.Fatalf("expected LastCheckedAt to be set")
			}
			if resource.Status.LastCheckedAt.Before(before.Add(-1 * time.Second)) || resource.Status.LastCheckedAt.After(after.Add(1*time.Second)) {
				t.Fatalf("LastCheckedAt out of expected range: %s", resource.Status.LastCheckedAt)
			}

			if tt.expectFailureSubstr == "" {
				if resource.Status.FailureReason != "" {
					t.Fatalf("expected empty failure reason, got %q", resource.Status.FailureReason)
				}
			} else if !strings.Contains(resource.Status.FailureReason, tt.expectFailureSubstr) {
				t.Fatalf("expected failure reason to contain %q, got %q", tt.expectFailureSubstr, resource.Status.FailureReason)
			}
		})
	}
}
