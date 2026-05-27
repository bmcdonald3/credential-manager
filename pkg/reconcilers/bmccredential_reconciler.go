// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
// This file contains user-customizable reconciliation logic for BmcCredential.
//
// ⚠️ This file is safe to edit - it will NOT be overwritten by code generation.
package reconcilers

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	res.Status.LastRotationUTC = time.Now().UTC().Format(time.RFC3339)
	res.Status.RotationSucceeded = false
	res.Status.FailureReason = ""

	if strings.TrimSpace(res.Spec.Address) == "" || strings.TrimSpace(res.Spec.TargetAccount) == "" || strings.TrimSpace(res.Spec.NodeIdentifier) == "" {
		err := fmt.Errorf("address, targetAccount, and nodeIdentifier are required")
		res.Status.FailureReason = err.Error()
		return err
	}

	store := r.secretStore()
	if store == nil {
		err := fmt.Errorf("secret store is not configured")
		res.Status.FailureReason = err.Error()
		return err
	}

	currentPassword, err := store.Read(res.Spec.NodeIdentifier)
	if err != nil {
		res.Status.FailureReason = err.Error()
		return err
	}

	newPassword, err := generateSecurePassword(24)
	if err != nil {
		res.Status.FailureReason = err.Error()
		return err
	}

	patchURL := fmt.Sprintf(
		"https://%s/redfish/v1/AccountService/Accounts/%s",
		strings.TrimSpace(res.Spec.Address),
		url.PathEscape(strings.TrimSpace(res.Spec.TargetAccount)),
	)

	payload, err := json.Marshal(map[string]string{"Password": newPassword})
	if err != nil {
		res.Status.FailureReason = err.Error()
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, bytes.NewReader(payload))
	if err != nil {
		res.Status.FailureReason = err.Error()
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(strings.TrimSpace(res.Spec.TargetAccount), currentPassword)

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		if isTimeoutErr(err) {
			err = fmt.Errorf("request timeout: %w", err)
		}
		res.Status.FailureReason = err.Error()
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		bodyText := strings.TrimSpace(string(body))
		if bodyText != "" {
			err = fmt.Errorf("redfish PATCH failed with status %d: %s", resp.StatusCode, bodyText)
		} else {
			err = fmt.Errorf("redfish PATCH failed with status %d", resp.StatusCode)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			err = fmt.Errorf("401 Unauthorized: %w", err)
		}
		res.Status.FailureReason = err.Error()
		return err
	}

	if err := store.Update(res.Spec.NodeIdentifier, newPassword); err != nil {
		err = fmt.Errorf("failed to persist updated credential: %w", err)
		res.Status.FailureReason = err.Error()
		return err
	}

	res.Status.RotationSucceeded = true
	res.Status.FailureReason = ""
	return nil
}

func generateSecurePassword(byteLen int) (string, error) {
	if byteLen <= 0 {
		return "", fmt.Errorf("password length must be positive")
	}

	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random password: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func isTimeoutErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var nerr net.Error
	return errors.As(err, &nerr) && nerr.Timeout()
}
