// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package v1

import (
	"context"
	"github.com/openchami/fabrica/pkg/fabrica"
	"time"
)

// BmcCredential represents a bmccredential resource
type BmcCredential struct {
	APIVersion string              `json:"apiVersion"`
	Kind       string              `json:"kind"`
	Metadata   fabrica.Metadata    `json:"metadata"`
	Spec       BmcCredentialSpec   `json:"spec" validate:"required"`
	Status     BmcCredentialStatus `json:"status,omitempty"`
}

// BmcCredentialSpec defines the desired state of BmcCredential
type BmcCredentialSpec struct {
	TargetAddress   string `json:"targetAddress" validate:"required"`
	TargetAccount   string `json:"targetAccount" validate:"required"`
	CurrentUsername string `json:"currentUsername" validate:"required"`
	CurrentPassword string `json:"currentPassword" validate:"required"`
	NewPassword     string `json:"newPassword" validate:"required"`
	RotationTrigger string `json:"rotationTrigger,omitempty"`
}

// BmcCredentialStatus defines the observed state of BmcCredential
type BmcCredentialStatus struct {
	LastRotationAttempt  *time.Time `json:"lastRotationAttempt,omitempty"`
	RotationSucceeded    bool       `json:"rotationSucceeded"`
	FailureReason        string     `json:"failureReason,omitempty"`
	ObservedTriggerValue string     `json:"observedTriggerValue,omitempty"`
}

// Validate implements custom validation logic for BmcCredential
func (r *BmcCredential) Validate(ctx context.Context) error {
	return nil
}

// GetKind returns the kind of the resource
func (r *BmcCredential) GetKind() string {
	return "BmcCredential"
}

// GetName returns the name of the resource
func (r *BmcCredential) GetName() string {
	return r.Metadata.Name
}

// GetUID returns the UID of the resource
func (r *BmcCredential) GetUID() string {
	return r.Metadata.UID
}

// IsHub marks this as the hub/storage version
func (r *BmcCredential) IsHub() {}
