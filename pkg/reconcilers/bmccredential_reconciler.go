// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
// This file contains user-customizable reconciliation logic for BmcCredential.
//
// ⚠️ This file is safe to edit - it will NOT be overwritten by code generation.
package reconcilers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/user/credential-manager/apis/example.fabrica.dev/v1"
)

// reconcileBmcCredential contains custom reconciliation logic.
//
// This method is called by the generated Reconcile() orchestration method.
// Implement BmcCredential-specific reconciliation logic here.
//
// Guidelines:
//  1. Keep this method idempotent (safe to call multiple times)
//  2. Update Status fields to reflect observed state
//  3. Emit events for significant state changes using r.EmitEvent()
//  4. Use r.Logger for debugging (Infof, Warnf, Errorf, Debugf)
//  5. Return errors for transient failures (will retry with backoff)
//  6. Access storage via r.Client (Get, List, Update, Create, Delete)
//
// Example implementation patterns:
//
// For hardware resources (BMC, Node):
//   - Connect to hardware endpoint
//   - Query current state
//   - Update Status.Connected, Status.Version, Status.Health
//   - Emit events when state changes
//
// For hierarchical resources (Rack, Chassis):
//   - Create/reconcile child resources
//   - Update Status with child counts and references
//   - Emit events when topology changes
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - res: The BmcCredential resource to reconcile
//
// Returns:
//   - error: If reconciliation failed (will trigger retry with backoff)
func (r *BmcCredentialReconciler) reconcileBmcCredential(ctx context.Context, res *v1.BmcCredential) error {
	_ = ctx

	res.Status.LastUpdatedAtUTC = time.Now().UTC()

	if err := validateBmcCredentialSecretIDs(res.Spec); err != nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = err.Error()
		r.Logger.Warnf("BmcCredential validation failed for %s: %v", res.GetUID(), err)
		return nil
	}

	res.Status.State = "Validated"
	res.Status.ValidationFailureReason = ""
	r.Logger.Infof("BmcCredential validated successfully for %s", res.GetUID())

	return nil
}

func validateBmcCredentialSecretIDs(spec v1.BmcCredentialSpec) error {
	if strings.TrimSpace(spec.CurrentPasswordSecretID) == "" {
		return fmt.Errorf("currentPasswordSecretID must not be empty")
	}

	if strings.TrimSpace(spec.NewPasswordSecretID) == "" {
		return fmt.Errorf("newPasswordSecretID must not be empty")
	}

	return nil
}
