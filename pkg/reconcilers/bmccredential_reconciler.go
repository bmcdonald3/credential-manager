// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT
// This file contains user-customizable reconciliation logic for BmcCredential.
//
// ⚠️ This file is safe to edit - it will NOT be overwritten by code generation.
package reconcilers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/openchami/fabrica/pkg/events"
	"github.com/openchami/fabrica/pkg/reconcile"
	"github.com/user/credential-manager/apis/example.fabrica.dev/v1"
)

type SecretResolver interface {
	LookupSecret(ctx context.Context, id string) (string, error)
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewBmcCredentialReconciler(
	client reconcile.ClientInterface,
	eventBus events.EventBus,
	secretStore SecretResolver,
	httpClient HTTPDoer,
) *BmcCredentialReconciler {
	r := NewDefaultBmcCredentialReconciler(client, eventBus)
	r.SecretStore = secretStore
	r.HTTPClient = httpClient
	return r
}

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
	res.Status.LastUpdatedAtUTC = time.Now().UTC()

	if err := validateBmcCredentialSecretIDs(res.Spec); err != nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = err.Error()
		r.Logger.Warnf("BmcCredential validation failed for %s: %v", res.GetUID(), err)
		return nil
	}

	if r.SecretStore == nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = "secret store is not configured"
		res.Status.LastUpdatedAtUTC = time.Now().UTC()
		r.Logger.Errorf("BmcCredential secret store missing for %s", res.GetUID())
		return nil
	}

	currentPassword, err := r.SecretStore.LookupSecret(ctx, res.Spec.CurrentPasswordSecretID)
	if err != nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = fmt.Sprintf("failed to resolve currentPasswordSecretID %q: %v", res.Spec.CurrentPasswordSecretID, err)
		res.Status.LastUpdatedAtUTC = time.Now().UTC()
		r.Logger.Warnf("BmcCredential current secret resolution failed for %s: %v", res.GetUID(), err)
		return nil
	}

	newPassword, err := r.SecretStore.LookupSecret(ctx, res.Spec.NewPasswordSecretID)
	if err != nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = fmt.Sprintf("failed to resolve newPasswordSecretID %q: %v", res.Spec.NewPasswordSecretID, err)
		res.Status.LastUpdatedAtUTC = time.Now().UTC()
		r.Logger.Warnf("BmcCredential new secret resolution failed for %s: %v", res.GetUID(), err)
		return nil
	}

	requestURL := buildRedfishAccountURL(res.Spec.TargetAddress, res.Spec.TargetAccount)
	payload, err := json.Marshal(map[string]string{"Password": newPassword})
	if err != nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = fmt.Sprintf("failed to marshal Redfish payload: %v", err)
		res.Status.LastUpdatedAtUTC = time.Now().UTC()
		return nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPatch, requestURL, strings.NewReader(string(payload)))
	if err != nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = fmt.Sprintf("failed to build Redfish PATCH request: %v", err)
		res.Status.LastUpdatedAtUTC = time.Now().UTC()
		return nil
	}
	req.SetBasicAuth(res.Spec.CurrentUsername, currentPassword)
	req.Header.Set("Content-Type", "application/json")

	httpClient := r.HTTPClient
	if httpClient == nil {
		httpClient = defaultInsecureHTTPClient()
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		res.Status.State = "Failed"
		res.Status.ValidationFailureReason = fmt.Sprintf("redfish patch request failed: %v", err)
		res.Status.LastUpdatedAtUTC = time.Now().UTC()
		r.Logger.Warnf("BmcCredential Redfish PATCH failed for %s: %v", res.GetUID(), err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		res.Status.State = "Success"
		res.Status.ValidationFailureReason = ""
		res.Status.LastUpdatedAtUTC = time.Now().UTC()
		r.Logger.Infof("BmcCredential password rotation succeeded for %s", res.GetUID())
		return nil
	}

	res.Status.State = "Failed"
	res.Status.ValidationFailureReason = fmt.Sprintf("redfish patch failed: HTTP %d", resp.StatusCode)
	res.Status.LastUpdatedAtUTC = time.Now().UTC()
	r.Logger.Warnf("BmcCredential Redfish PATCH rejected for %s: HTTP %d", res.GetUID(), resp.StatusCode)

	return nil
}

func defaultInsecureHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 20 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func buildRedfishAccountURL(targetAddress string, targetAccount string) string {
	host := strings.TrimSpace(targetAddress)
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimSuffix(host, "/")

	account := url.PathEscape(strings.TrimSpace(targetAccount))
	return fmt.Sprintf("https://%s/redfish/v1/AccountService/Accounts/%s", host, account)
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
