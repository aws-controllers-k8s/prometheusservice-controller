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

package rule_groups_namespace

import (
	"context"
	"errors"
	"time"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go/service/prometheusservice"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

var (
	ErrRuleGroupsNamespaceCreating = errors.New("Rule Groups Namespace is in 'CREATING' state, cannot be modified or deleted")
	ErrRuleGroupsNamespaceDeleting = errors.New("Rule Groups Namespace is in 'DELETING' state, cannot be modified or deleted")
	ErrRuleGroupsNamespaceUpdating = errors.New("Rule Groups Namespace is in 'UPDATING' state, cannot be modified or deleted")
)

var (
	// TerminalStatuses are the status strings that are a rule groups namespace
	TerminalStatuses = []svcapitypes.RuleGroupsNamespaceStatusCode{
		svcapitypes.RuleGroupsNamespaceStatusCode_CREATION_FAILED,
		svcapitypes.RuleGroupsNamespaceStatusCode_UPDATE_FAILED,
		svcapitypes.RuleGroupsNamespaceStatusCode_DELETING,
	}
)

var (
	requeueWaitWhileDeleting = ackrequeue.NeededAfter(
		ErrRuleGroupsNamespaceDeleting,
		10*time.Second,
	)
	requeueWaitWhileCreating = ackrequeue.NeededAfter(
		ErrRuleGroupsNamespaceCreating,
		15*time.Second,
	)
	requeueWaitWhileUpdating = ackrequeue.NeededAfter(
		ErrRuleGroupsNamespaceUpdating,
		10*time.Second,
	)
)

// ruleGroupsNamespaceCreating returns true if the supplied rule groups is in the process
// of being created
func ruleGroupsNamespaceCreating(r *resource) bool {
	if r.ko.Status.Status == nil {
		return false
	}
	ws := *r.ko.Status.Status.StatusCode
	return ws == string(svcapitypes.RuleGroupsNamespaceStatusCode_CREATING)
}

// ruleGroupsNamespaceDeleting returns true if the supplied rule groups is in the process
// of being deleted
func ruleGroupsNamespaceDeleting(r *resource) bool {
	if r.ko.Status.Status == nil {
		return false
	}
	ws := *r.ko.Status.Status.StatusCode
	return ws == string(svcapitypes.RuleGroupsNamespaceStatusCode_DELETING)
}

// ruleGroupsNamespaceUpdating returns true if the supplied rule groups is in the process
// of being updated
func ruleGroupsNamespaceUpdating(r *resource) bool {
	if r.ko.Status.Status == nil {
		return false
	}
	ws := *r.ko.Status.Status.StatusCode
	return ws == string(svcapitypes.RuleGroupsNamespaceStatusCode_UPDATING)
}

// ruleGroupsNamespaceHasTerminalStatus returns whether the supplied rule groups namespace is in a
// terminal state
func ruleGroupsNamespaceHasTerminalStatus(r *resource) bool {
	if r.ko.Status.Status.StatusCode == nil {
		return false
	}
	ts := *r.ko.Status.Status.StatusCode
	for _, s := range TerminalStatuses {
		if ts == string(s) {
			return true
		}
	}
	return false
}

// customUpdateRuleGroupsNamespace patches each of the resource properties in the backend AWS
// service API and returns a new resource with updated fields.
func (rm *resourceManager) customUpdateRuleGroupsNamespace(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateRuleGroupsNamespace")
	defer exit(err)

	// Check if the state is being currently created, updated or deleted.
	// If it is, then requeue because we can't update while it is in those states.
	var sc string = ""
	if latest.ko.Status.Status != nil {
		sc = *latest.ko.Status.Status.StatusCode
	}
	switch sc {
	case string(svcapitypes.RuleGroupsNamespaceStatusCode_DELETING):
		return desired, requeueWaitWhileDeleting
	case string(svcapitypes.RuleGroupsNamespaceStatusCode_UPDATING):
		return desired, requeueWaitWhileUpdating
	case string(svcapitypes.RuleGroupsNamespaceStatusCode_CREATING):
		return desired, requeueWaitWhileCreating
	}

	if ruleGroupsNamespaceHasTerminalStatus(latest) {
		msg := "Rule Groups Namespace is in '" + *latest.ko.Status.Status.StatusCode + "' status"
		ackcondition.SetTerminal(desired, corev1.ConditionTrue, &msg, nil)
		ackcondition.SetSynced(desired, corev1.ConditionTrue, nil, nil)
		return desired, nil
	}

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

	rm.setStatusDefaults(ko)
	if delta.DifferentAt("Spec.Tags") {
		err = rm.updateRuleGroupsNamespaceTags(ctx, latest, desired)
		if err != nil {
			return nil, err
		}
	}

	if delta.DifferentAt("Spec.Configuration") {
		updatedResource, err := rm.updateRuleGroupsNamespace(ctx, desired)
		if err != nil {
			return nil, err
		}
		if updatedResource != nil {
			return updatedResource, nil
		}
	}

	return &resource{ko}, nil
}

// updateRuleGroupsNamespaceTags uses TagResource and UntagResource to add, remove and update
// rule groups namespace tags.
func (rm *resourceManager) updateRuleGroupsNamespaceTags(
	ctx context.Context,
	latest *resource,
	desired *resource,
) error {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.updateRuleGroupsNamespaceTags")
	defer exit(err)

	addedOrUpdated, removed := compareMaps(latest.ko.Spec.Tags, desired.ko.Spec.Tags)

	if len(removed) > 0 {
		removeTags := []*string{}
		for i := range removed {
			removeTags = append(removeTags, &removed[i])
		}

		input := &svcsdk.UntagResourceInput{
			ResourceArn: (*string)(desired.ko.Status.ACKResourceMetadata.ARN),
			TagKeys:     removeTags,
		}

		_, err = rm.sdkapi.UntagResourceWithContext(ctx, input)
		rm.metrics.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}

	if len(addedOrUpdated) > 0 {
		input := &svcsdk.TagResourceInput{
			ResourceArn: (*string)(desired.ko.Status.ACKResourceMetadata.ARN),
			Tags:        addedOrUpdated,
		}
		_, err = rm.sdkapi.TagResourceWithContext(ctx, input)
		rm.metrics.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
	}

	return nil

}

// updateRuleGroupsNamespace calls updateRuleGroupsNamespace to update a specific rules group
// namespace.
func (rm *resourceManager) updateRuleGroupsNamespace(
	ctx context.Context,
	desired *resource,
) (*resource, error) {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.updateRuleGroupsNamespace")
	defer exit(err)

	// Convert the string version of the configuration to a byte slice
	// because the API expects a base64 encoding. The conversion to base64
	// is handled automatically by k8s.
	if desired.ko.Spec.Configuration != nil {
		desired.ko.Spec.Data = []byte(*desired.ko.Spec.Configuration)
	}

	input := &svcsdk.PutRuleGroupsNamespaceInput{
		WorkspaceId: desired.ko.Spec.WorkspaceID,
		Name:        desired.ko.Spec.Name,
		Data:        desired.ko.Spec.Data,
	}

	resp, err := rm.sdkapi.PutRuleGroupsNamespaceWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "PutRuleGroupsNamespaceWithContext", err)
	if err != nil {
		return nil, err
	}

	// After the call, reset the Data field since it is not user facing.
	desired.ko.Spec.Data = nil

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
	// Some updates might be instant and the resource will remain in an active state.
	// While other updates, might take a while to update and the resource will be in an `UPDATING`
	// state. If this is the case, then we want to requeue until the resource is done updating.
	if ruleGroupsNamespaceUpdating(&resource{ko}) {
		// Setting resource synced condition to false will trigger a requeue of
		// the resource. No need to return a requeue error here.
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
		return &resource{ko}, nil
	}

	return &resource{ko}, nil

}

// compareMaps compares two string to string maps and returns two outputs: a
// map of the new and updated key/values observed, and a list of the keys of the
// removed values.
func compareMaps(
	a map[string]*string,
	b map[string]*string,
) (addedOrUpdated map[string]*string, removed []string) {
	addedOrUpdated = map[string]*string{}
	visited := make(map[string]bool, len(a))
	for keyA, valueA := range a {
		valueB, found := b[keyA]
		if !found {
			removed = append(removed, keyA)
			continue
		}
		if *valueA != *valueB {
			addedOrUpdated[keyA] = valueB
		}
		visited[keyA] = true
	}
	for keyB, valueB := range b {
		_, found := a[keyB]
		if !found {
			addedOrUpdated[keyB] = valueB
		}
	}
	return
}

func customPreCompare(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {

	if len(a.ko.Spec.Tags) != len(b.ko.Spec.Tags) {
		delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
	} else if a.ko.Spec.Tags != nil && b.ko.Spec.Tags != nil {
		if !ackcompare.MapStringStringPEqual(a.ko.Spec.Tags, b.ko.Spec.Tags) {
			delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
		}
	}
}
