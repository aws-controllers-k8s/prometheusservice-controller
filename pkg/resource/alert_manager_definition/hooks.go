// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package alert_manager_definition

import (
	"context"
	"errors"
	"strings"
	"time"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/amp"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

var (
	ErrAlertManagerDefinitionCreating = errors.New("Alert Manager Definition in 'CREATING' state, cannot be modified or deleted")
	ErrAlertManagerDefinitionDeleting = errors.New("Alert Manager Definition in 'DELETING' state, cannot be modified or deleted")
	ErrAlertManagerDefinitionUpdating = errors.New("Alert Manager Definition in 'UPDATING' state, cannot be modified or deleted")
)

var (
	requeueWaitWhileDeleting = ackrequeue.NeededAfter(
		ErrAlertManagerDefinitionDeleting,
		10*time.Second,
	)
	requeueWaitWhileCreating = ackrequeue.NeededAfter(
		ErrAlertManagerDefinitionCreating,
		15*time.Second,
	)
	requeueWaitWhileUpdating = ackrequeue.NeededAfter(
		ErrAlertManagerDefinitionUpdating,
		10*time.Second,
	)
)

// alertManagerDefinitionCreating returns true if the supplied definition
// is in the process of being created
func alertManagerDefinitionCreating(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(svcapitypes.AlertManagerDefinitionStatusCode_CREATING)
}

// alertManagerDefinitionDeleting returns true if the supplied definition
// is in the process of being deleted
func alertManagerDefinitionDeleting(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(svcapitypes.AlertManagerDefinitionStatusCode_DELETING)
}

// alertManagerDefinitionUpdating returns true if the supplied definition
// is in the process of being updated
func alertManagerDefinitionUpdating(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(svcapitypes.AlertManagerDefinitionStatusCode_UPDATING)
}

// alertManagerDefinitionActive returns true if the supplied
// definition is in an active state
func alertManagerDefinitionActive(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(svcapitypes.AlertManagerDefinitionStatusCode_ACTIVE)
}

// alertManagerDefinitionStatusFailed returns true if the supplied definition
// has a status of creation failed  or update failed
func alertManagerDefinitionStatusFailed(r *resource) bool {
	if r.ko.Status.StatusCode == nil {
		return false
	}
	ws := *r.ko.Status.StatusCode
	return ws == string(svcapitypes.AlertManagerDefinitionStatusCode_CREATION_FAILED) || ws == string(svcapitypes.AlertManagerDefinitionStatusCode_UPDATE_FAILED)
}

// alertManagerDefinitionValidationError returns true if the Status Reason
// includes a validation error as part of the reason for the status.
func alertManagerDefinitionValidationError(r *resource) bool {
	if r.ko.Status.StatusReason == nil {
		return false
	}
	ws := *r.ko.Status.StatusReason
	return strings.Contains(ws, "error validating")
}

// customUpdateAlertManagerDefinition patches each of the resource properties in the backend AWS
// service API and returns a new resource with updated fields.
func (rm *resourceManager) customUpdateAlertManagerDefinition(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateAlertManagerDefinition")
	defer func() {
		exit(err)
	}()

	// Check if the state is being currently created, updated or deleted.
	// If it is, then requeue because we can't update while it is in those states.
	// For failed states (create & update) and active states, the user can
	// still update the alert manager definition.
	var sc *string = latest.ko.Status.StatusCode
	switch *sc {
	case string(svcapitypes.AlertManagerDefinitionStatusCode_DELETING):
		return desired, requeueWaitWhileDeleting
	case string(svcapitypes.AlertManagerDefinitionStatusCode_UPDATING):
		return desired, requeueWaitWhileUpdating
	case string(svcapitypes.AlertManagerDefinitionStatusCode_CREATING):
		return desired, requeueWaitWhileCreating
	}

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()
	rm.setStatusDefaults(ko)
	if delta.DifferentAt("Spec.Configuration") {
		updatedResource, err := rm.updateAlertManagerDefinitionData(ctx, desired)
		if err != nil {
			return nil, err
		}
		return updatedResource, nil

	}

	return &resource{ko}, nil

}

// updateAlertManagerDefinition calls updateAlertManagerDefinitionData to update the
// alert manager configuration field.
func (rm *resourceManager) updateAlertManagerDefinitionData(
	ctx context.Context,
	desired *resource,
) (*resource, error) {
	{
		var err error
		rlog := ackrtlog.FromContext(ctx)
		exit := rlog.Trace("rm.updateAlertManagerDefinitionData")
		defer func() {
			exit(err)
		}()

		var configurationBytes []byte = nil
		// Convert the string version of the definition to a byte slice
		// because the API expects a base64 encoding. The conversion to base64
		// is handled automatically by k8s.
		if desired.ko.Spec.Configuration != nil {
			configurationBytes = []byte(*desired.ko.Spec.Configuration)
		}

		input := &svcsdk.PutAlertManagerDefinitionInput{
			Data:        configurationBytes,
			WorkspaceId: desired.ko.Spec.WorkspaceID,
		}

		resp, err := rm.sdkapi.PutAlertManagerDefinition(ctx, input)
		rm.metrics.RecordAPICall("UPDATE", "putAlertManagerDefinition", err)
		if err != nil {
			return nil, err
		}

		ko := desired.ko.DeepCopy()

		// Check the status of the alert manager definition
		if resp.Status != nil {
			if resp.Status.StatusCode != "" {
				ko.Status.StatusCode = aws.String(string(resp.Status.StatusCode))
			} else {
				ko.Status.StatusCode = nil
			}
			if resp.Status.StatusReason != nil {
				ko.Status.StatusReason = resp.Status.StatusReason
			} else {
				ko.Status.StatusReason = nil
			}
		} else {
			ko.Status.StatusCode = nil
			ko.Status.StatusReason = nil

		}

		rm.setStatusDefaults(ko)
		// Some updates might be instant and the resource will remain in an active state.
		// While other updates, might take a while to update and the resource will be in an `UPDATING`
		// state. If this is the case, then we want to requeue until the resource is done updating.
		if alertManagerDefinitionUpdating(&resource{ko}) {
			// Setting resource synced condition to false will trigger a requeue of
			// the resource. No need to return a requeue error here.
			ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
			return &resource{ko}, nil
		}

		return &resource{ko}, nil
	}
}
