package reconcilers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1 "github.com/user/bmc-manager/apis/example.fabrica.dev/v1"
)

func TestRotateBMCPasswordWithTimeout(t *testing.T) {
	testCases := []struct {
		name             string
		handler          http.HandlerFunc
		spec             v1.BmcCredentialSpec
		timeout          time.Duration
		expectedErrSub   string
		expectedPath     string
		expectedPassword string
	}{
		{
			name: "success status 200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
			spec: v1.BmcCredentialSpec{
				CurrentUsername: "admin",
				CurrentPassword: "old-pass",
				TargetAccount:   "2",
				NewPassword:     "new-pass",
			},
			timeout:          2 * time.Second,
			expectedPath:     "/redfish/v1/AccountService/Accounts/2",
			expectedPassword: "new-pass",
		},
		{
			name: "success status 204",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			},
			spec: v1.BmcCredentialSpec{
				CurrentUsername: "admin",
				CurrentPassword: "old-pass",
				TargetAccount:   "admin user",
				NewPassword:     "new-pass-2",
			},
			timeout:          2 * time.Second,
			expectedPath:     "/redfish/v1/AccountService/Accounts/admin%20user",
			expectedPassword: "new-pass-2",
		},
		{
			name: "unauthorized",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusUnauthorized)
			},
			spec: v1.BmcCredentialSpec{
				CurrentUsername: "bad-user",
				CurrentPassword: "bad-pass",
				TargetAccount:   "3",
				NewPassword:     "ignored",
			},
			timeout:        2 * time.Second,
			expectedErrSub: "unauthorized",
			expectedPath:   "/redfish/v1/AccountService/Accounts/3",
		},
		{
			name: "timeout",
			handler: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(250 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			},
			spec: v1.BmcCredentialSpec{
				CurrentUsername: "admin",
				CurrentPassword: "old-pass",
				TargetAccount:   "4",
				NewPassword:     "new-pass",
			},
			timeout:        50 * time.Millisecond,
			expectedErrSub: "context deadline exceeded",
			expectedPath:   "/redfish/v1/AccountService/Accounts/4",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			var gotPassword string
			var gotUser string
			var gotPass string

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.EscapedPath()
				gotUser, gotPass, _ = r.BasicAuth()
				body, err := io.ReadAll(r.Body)
				if err == nil && strings.Contains(string(body), "\"Password\":") {
					gotPassword = string(body)
				}
				tc.handler(w, r)
			}))
			defer srv.Close()

			tc.spec.TargetAddress = strings.TrimPrefix(srv.URL, "https://")
			err := rotateBMCPasswordWithTimeout(context.Background(), tc.spec, tc.timeout)

			if tc.expectedErrSub == "" && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tc.expectedErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q but got nil", tc.expectedErrSub)
				}
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.expectedErrSub)) {
					t.Fatalf("expected error containing %q, got %q", tc.expectedErrSub, err.Error())
				}
			}

			if gotPath != tc.expectedPath {
				t.Fatalf("expected path %q, got %q", tc.expectedPath, gotPath)
			}
			if gotUser != tc.spec.CurrentUsername || gotPass != tc.spec.CurrentPassword {
				t.Fatalf("unexpected basic auth credentials got (%q, %q)", gotUser, gotPass)
			}
			if tc.expectedPassword != "" && !strings.Contains(gotPassword, fmt.Sprintf("\"Password\":\"%s\"", tc.expectedPassword)) {
				t.Fatalf("expected payload to include new password, got %q", gotPassword)
			}
		})
	}
}
