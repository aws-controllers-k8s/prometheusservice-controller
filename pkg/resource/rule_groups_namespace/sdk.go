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

// Code generated by ack-generate. DO NOT EDIT.

package rule_groups_namespace

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go/aws"
	svcsdk "github.com/aws/aws-sdk-go/service/prometheusservice"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

// Hack to avoid import errors during build...
var (
	_ = &metav1.Time{}
	_ = strings.ToLower("")
	_ = &aws.JSONValue{}
	_ = &svcsdk.PrometheusService{}
	_ = &svcapitypes.RuleGroupsNamespace{}
	_ = ackv1alpha1.AWSAccountID("")
	_ = &ackerr.NotFound
	_ = &ackcondition.NotManagedMessage
	_ = &reflect.Value{}
	_ = fmt.Sprintf("")
	_ = &ackrequeue.NoRequeue{}
)

// sdkFind returns SDK-specific information about a supplied resource
func (rm *resourceManager) sdkFind(
	ctx context.Context,
	r *resource,
) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkFind")
	defer func() {
		exit(err)
	}()
	// If any required fields in the input shape are missing, AWS resource is
	// not created yet. Return NotFound here to indicate to callers that the
	// resource isn't yet created.
	if rm.requiredFieldsMissingFromReadOneInput(r) {
		return nil, ackerr.NotFound
	}

	input, err := rm.newDescribeRequestPayload(r)
	if err != nil {
		return nil, err
	}

	var resp *svcsdk.DescribeRuleGroupsNamespaceOutput
	resp, err = rm.sdkapi.DescribeRuleGroupsNamespaceWithContext(ctx, input)
	rm.metrics.RecordAPICall("READ_ONE", "DescribeRuleGroupsNamespace", err)
	if err != nil {
		if awsErr, ok := ackerr.AWSError(err); ok && awsErr.Code() == "ResourceNotFoundException" {
			return nil, ackerr.NotFound
		}
		return nil, err
	}

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := r.ko.DeepCopy()

	// The data field stores the base64 encoding of the rule groups namespace.
	// However, to make the CR's more user friendly, we convert the base64 encoding to a
	// string. We store it in a custom created field.
	if resp.RuleGroupsNamespace.Data != nil {
		// Convert the base64 byte array to a human-readable string
		ruleGroupsNamespaceDataString := string(resp.RuleGroupsNamespace.Data)
		ko.Spec.Configuration = &ruleGroupsNamespaceDataString

		// Remove the data field as it is not user facing
		resp.RuleGroupsNamespace.Data = nil
	} else {
		ko.Spec.Configuration = nil
	}

	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if resp.RuleGroupsNamespace.Arn != nil {
		arn := ackv1alpha1.AWSResourceName(*resp.RuleGroupsNamespace.Arn)
		ko.Status.ACKResourceMetadata.ARN = &arn
	}
	if resp.RuleGroupsNamespace.Name != nil {
		ko.Spec.Name = resp.RuleGroupsNamespace.Name
	} else {
		ko.Spec.Name = nil
	}
	if resp.RuleGroupsNamespace.Status != nil {
		f5 := &svcapitypes.RuleGroupsNamespaceStatus_SDK{}
		if resp.RuleGroupsNamespace.Status.StatusCode != nil {
			f5.StatusCode = resp.RuleGroupsNamespace.Status.StatusCode
		}
		if resp.RuleGroupsNamespace.Status.StatusReason != nil {
			f5.StatusReason = resp.RuleGroupsNamespace.Status.StatusReason
		}
		ko.Status.Status = f5
	} else {
		ko.Status.Status = nil
	}
	if resp.RuleGroupsNamespace.Tags != nil {
		f6 := map[string]*string{}
		for f6key, f6valiter := range resp.RuleGroupsNamespace.Tags {
			var f6val string
			f6val = *f6valiter
			f6[f6key] = &f6val
		}
		ko.Spec.Tags = f6
	} else {
		ko.Spec.Tags = nil
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}

// requiredFieldsMissingFromReadOneInput returns true if there are any fields
// for the ReadOne Input shape that are required but not present in the
// resource's Spec or Status
func (rm *resourceManager) requiredFieldsMissingFromReadOneInput(
	r *resource,
) bool {
	return r.ko.Spec.Name == nil || r.ko.Spec.WorkspaceID == nil

}

// newDescribeRequestPayload returns SDK-specific struct for the HTTP request
// payload of the Describe API call for the resource
func (rm *resourceManager) newDescribeRequestPayload(
	r *resource,
) (*svcsdk.DescribeRuleGroupsNamespaceInput, error) {
	res := &svcsdk.DescribeRuleGroupsNamespaceInput{}

	if r.ko.Spec.Name != nil {
		res.SetName(*r.ko.Spec.Name)
	}
	if r.ko.Spec.WorkspaceID != nil {
		res.SetWorkspaceId(*r.ko.Spec.WorkspaceID)
	}

	return res, nil
}

// sdkCreate creates the supplied resource in the backend AWS service API and
// returns a copy of the resource with resource fields (in both Spec and
// Status) filled in with values from the CREATE API operation's Output shape.
func (rm *resourceManager) sdkCreate(
	ctx context.Context,
	desired *resource,
) (created *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkCreate")
	defer func() {
		exit(err)
	}()
	input, err := rm.newCreateRequestPayload(ctx, desired)
	if err != nil {
		return nil, err
	}

	// Convert the string version of the configuration to a byte slice
	// because the API expects a base64 encoding. The conversion to base64
	// is handled automatically by k8s.
	if desired.ko.Spec.Configuration != nil {
		input.Data = []byte(*desired.ko.Spec.Configuration)
	}

	var resp *svcsdk.CreateRuleGroupsNamespaceOutput
	_ = resp
	resp, err = rm.sdkapi.CreateRuleGroupsNamespaceWithContext(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "CreateRuleGroupsNamespace", err)
	if err != nil {
		return nil, err
	}
	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if resp.Arn != nil {
		arn := ackv1alpha1.AWSResourceName(*resp.Arn)
		ko.Status.ACKResourceMetadata.ARN = &arn
	}
	if resp.Name != nil {
		ko.Spec.Name = resp.Name
	} else {
		ko.Spec.Name = nil
	}
	if resp.Status != nil {
		f2 := &svcapitypes.RuleGroupsNamespaceStatus_SDK{}
		if resp.Status.StatusCode != nil {
			f2.StatusCode = resp.Status.StatusCode
		}
		if resp.Status.StatusReason != nil {
			f2.StatusReason = resp.Status.StatusReason
		}
		ko.Status.Status = f2
	} else {
		ko.Status.Status = nil
	}
	if resp.Tags != nil {
		f3 := map[string]*string{}
		for f3key, f3valiter := range resp.Tags {
			var f3val string
			f3val = *f3valiter
			f3[f3key] = &f3val
		}
		ko.Spec.Tags = f3
	} else {
		ko.Spec.Tags = nil
	}

	rm.setStatusDefaults(ko)

	// We expect the rule group to be in 'creating' status since we just
	// issued the call to create it, but I suppose it doesn't hurt to check
	// here.
	if ruleGroupsNamespaceCreating(&resource{ko}) {
		// Setting resource synced condition to false will trigger a requeue of
		// the resource. No need to return a requeue error here.
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
		return &resource{ko}, nil
	}

	return &resource{ko}, nil
}

// newCreateRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Create API call for the resource
func (rm *resourceManager) newCreateRequestPayload(
	ctx context.Context,
	r *resource,
) (*svcsdk.CreateRuleGroupsNamespaceInput, error) {
	res := &svcsdk.CreateRuleGroupsNamespaceInput{}

	if r.ko.Spec.Name != nil {
		res.SetName(*r.ko.Spec.Name)
	}
	if r.ko.Spec.Tags != nil {
		f1 := map[string]*string{}
		for f1key, f1valiter := range r.ko.Spec.Tags {
			var f1val string
			f1val = *f1valiter
			f1[f1key] = &f1val
		}
		res.SetTags(f1)
	}
	if r.ko.Spec.WorkspaceID != nil {
		res.SetWorkspaceId(*r.ko.Spec.WorkspaceID)
	}

	return res, nil
}

// sdkUpdate patches the supplied resource in the backend AWS service API and
// returns a new resource with updated fields.
func (rm *resourceManager) sdkUpdate(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	return rm.customUpdateRuleGroupsNamespace(ctx, desired, latest, delta)
}

// sdkDelete deletes the supplied resource in the backend AWS service API
func (rm *resourceManager) sdkDelete(
	ctx context.Context,
	r *resource,
) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkDelete")
	defer func() {
		exit(err)
	}()
	input, err := rm.newDeleteRequestPayload(r)
	if err != nil {
		return nil, err
	}
	var resp *svcsdk.DeleteRuleGroupsNamespaceOutput
	_ = resp
	resp, err = rm.sdkapi.DeleteRuleGroupsNamespaceWithContext(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "DeleteRuleGroupsNamespace", err)
	return nil, err
}

// newDeleteRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Delete API call for the resource
func (rm *resourceManager) newDeleteRequestPayload(
	r *resource,
) (*svcsdk.DeleteRuleGroupsNamespaceInput, error) {
	res := &svcsdk.DeleteRuleGroupsNamespaceInput{}

	if r.ko.Spec.Name != nil {
		res.SetName(*r.ko.Spec.Name)
	}
	if r.ko.Spec.WorkspaceID != nil {
		res.SetWorkspaceId(*r.ko.Spec.WorkspaceID)
	}

	return res, nil
}

// setStatusDefaults sets default properties into supplied custom resource
func (rm *resourceManager) setStatusDefaults(
	ko *svcapitypes.RuleGroupsNamespace,
) {
	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if ko.Status.ACKResourceMetadata.Region == nil {
		ko.Status.ACKResourceMetadata.Region = &rm.awsRegion
	}
	if ko.Status.ACKResourceMetadata.OwnerAccountID == nil {
		ko.Status.ACKResourceMetadata.OwnerAccountID = &rm.awsAccountID
	}
	if ko.Status.Conditions == nil {
		ko.Status.Conditions = []*ackv1alpha1.Condition{}
	}
}

// updateConditions returns updated resource, true; if conditions were updated
// else it returns nil, false
func (rm *resourceManager) updateConditions(
	r *resource,
	onSuccess bool,
	err error,
) (*resource, bool) {
	ko := r.ko.DeepCopy()
	rm.setStatusDefaults(ko)

	// Terminal condition
	var terminalCondition *ackv1alpha1.Condition = nil
	var recoverableCondition *ackv1alpha1.Condition = nil
	var syncCondition *ackv1alpha1.Condition = nil
	for _, condition := range ko.Status.Conditions {
		if condition.Type == ackv1alpha1.ConditionTypeTerminal {
			terminalCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeRecoverable {
			recoverableCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeResourceSynced {
			syncCondition = condition
		}
	}
	var termError *ackerr.TerminalError
	if rm.terminalAWSError(err) || err == ackerr.SecretTypeNotSupported || err == ackerr.SecretNotFound || errors.As(err, &termError) {
		if terminalCondition == nil {
			terminalCondition = &ackv1alpha1.Condition{
				Type: ackv1alpha1.ConditionTypeTerminal,
			}
			ko.Status.Conditions = append(ko.Status.Conditions, terminalCondition)
		}
		var errorMessage = ""
		if err == ackerr.SecretTypeNotSupported || err == ackerr.SecretNotFound || errors.As(err, &termError) {
			errorMessage = err.Error()
		} else {
			awsErr, _ := ackerr.AWSError(err)
			errorMessage = awsErr.Error()
		}
		terminalCondition.Status = corev1.ConditionTrue
		terminalCondition.Message = &errorMessage
	} else {
		// Clear the terminal condition if no longer present
		if terminalCondition != nil {
			terminalCondition.Status = corev1.ConditionFalse
			terminalCondition.Message = nil
		}
		// Handling Recoverable Conditions
		if err != nil {
			if recoverableCondition == nil {
				// Add a new Condition containing a non-terminal error
				recoverableCondition = &ackv1alpha1.Condition{
					Type: ackv1alpha1.ConditionTypeRecoverable,
				}
				ko.Status.Conditions = append(ko.Status.Conditions, recoverableCondition)
			}
			recoverableCondition.Status = corev1.ConditionTrue
			awsErr, _ := ackerr.AWSError(err)
			errorMessage := err.Error()
			if awsErr != nil {
				errorMessage = awsErr.Error()
			}
			recoverableCondition.Message = &errorMessage
		} else if recoverableCondition != nil {
			recoverableCondition.Status = corev1.ConditionFalse
			recoverableCondition.Message = nil
		}
	}
	// Required to avoid the "declared but not used" error in the default case
	_ = syncCondition
	if terminalCondition != nil || recoverableCondition != nil || syncCondition != nil {
		return &resource{ko}, true // updated
	}
	return nil, false // not updated
}

// terminalAWSError returns awserr, true; if the supplied error is an aws Error type
// and if the exception indicates that it is a Terminal exception
// 'Terminal' exception are specified in generator configuration
func (rm *resourceManager) terminalAWSError(err error) bool {
	if err == nil {
		return false
	}
	awsErr, ok := ackerr.AWSError(err)
	if !ok {
		return false
	}
	switch awsErr.Code() {
	case "ValidationException":
		return true
	default:
		return false
	}
}

// getImmutableFieldChanges returns list of immutable fields from the
func (rm *resourceManager) getImmutableFieldChanges(
	delta *ackcompare.Delta,
) []string {
	var fields []string
	if delta.DifferentAt("Spec.Name") {
		fields = append(fields, "Name")
	}
	if delta.DifferentAt("Spec.WorkspaceID") {
		fields = append(fields, "WorkspaceID")
	}

	return fields
}
