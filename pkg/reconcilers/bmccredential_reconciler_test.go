package reconcilers

import (
	"context"
	"testing"

	"github.com/openchami/fabrica/pkg/reconcile"
	"github.com/user/credential-manager/apis/example.fabrica.dev/v1"
)

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
		name       string
		spec       v1.BmcCredentialSpec
		wantState  string
		wantReason string
	}{
		{
			name: "validated when both secret IDs are present",
			spec: v1.BmcCredentialSpec{
				CurrentPasswordSecretID: "current-secret",
				NewPasswordSecretID:     "new-secret",
			},
			wantState: "Validated",
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
			name: "failed when new secret ID is missing",
			spec: v1.BmcCredentialSpec{
				CurrentPasswordSecretID: "current-secret",
				NewPasswordSecretID:     "",
			},
			wantState:  "Failed",
			wantReason: "newPasswordSecretID must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &BmcCredentialReconciler{
				BaseReconciler: reconcile.BaseReconciler{Logger: reconcile.NewDefaultLogger()},
			}

			res := &v1.BmcCredential{Spec: tt.spec}
			if err := reconciler.reconcileBmcCredential(context.Background(), res); err != nil {
				t.Fatalf("expected no reconcile error, got %v", err)
			}

			if res.Status.State != tt.wantState {
				t.Fatalf("expected state %q, got %q", tt.wantState, res.Status.State)
			}

			if res.Status.ValidationFailureReason != tt.wantReason {
				t.Fatalf("expected reason %q, got %q", tt.wantReason, res.Status.ValidationFailureReason)
			}

			if res.Status.LastUpdatedAtUTC.IsZero() {
				t.Fatal("expected lastUpdatedAtUTC to be set")
			}
		})
	}
}
