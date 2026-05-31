package reconcilers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/openchami/credential-manager/apis/credentials.openchami.org/v1"
)

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

func TestRotateRedfishPassword(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		responseBody   string
		wantSuccess    bool
		wantFailureSub string
	}{
		{
			name:         "http 204 is success",
			statusCode:   http.StatusNoContent,
			responseBody: "",
			wantSuccess:  true,
		},
		{
			name:           "http 401 is failure",
			statusCode:     http.StatusUnauthorized,
			responseBody:   "invalid credentials",
			wantSuccess:    false,
			wantFailureSub: "received HTTP 401",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

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

			spec := v1.BmcCredentialSpec{
				TargetAddress:   targetURL.Host,
				TargetAccount:   "root",
				CurrentUsername: "admin",
				CurrentPassword: "oldpass",
				NewPassword:     "newpass",
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
		})
	}
}
