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

package workspace

import (
	"context"
	"errors"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/amp"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

// workspaceCreating returns true if the supplied workspace is in the process
// of being created
func workspaceCreating(r *resource) bool {
	if r.ko.Status.Status == nil {
		return false
	}
	ws := *r.ko.Status.Status.StatusCode
	return ws == string(svcapitypes.WorkspaceStatusCode_CREATING)
}

// workspaceCreating returns true if the supplied workspace is in an active state
func workspaceActive(r *resource) bool {
	if r.ko.Status.Status == nil {
		return false
	}
	ws := *r.ko.Status.Status.StatusCode
	return ws == string(svcapitypes.WorkspaceStatusCode_ACTIVE)
}

// customUpdateWorkspace patches each of the resource properties in the backend AWS
// service API and returns a new resource with updated fields.
func (rm *resourceManager) customUpdateWorkspace(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {

	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateWorkspace")
	defer exit(err)

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

	rm.setStatusDefaults(ko)

	// Check if the state is active before updating
	if !workspaceActive(latest) {
		msg := "Cannot update workspace as it is not active, current status=" + string(*latest.ko.Status.Status.StatusCode)
		ackcondition.SetSynced(desired, corev1.ConditionFalse, &msg, nil)
		return desired, ackrequeue.NeededAfter(
			errors.New(msg),
			ackrequeue.DefaultRequeueAfterDuration,
		)
	}

	if delta.DifferentAt("Spec.Tags") {
		err = rm.updateWorkspaceTags(ctx, latest, desired)
		if err != nil {
			return nil, err
		}
	}

	if delta.DifferentAt("Spec.Alias") {
		err = rm.updateWorkspaceAlias(ctx, desired)
		if err != nil {
			return nil, err
		}
	}

	return &resource{ko}, nil
}

// updateWorkspaceTags uses TagResource and UntagResource to add, remove and update
// workspace tags.
func (rm *resourceManager) updateWorkspaceTags(
	ctx context.Context,
	latest *resource,
	desired *resource,
) error {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.updateWorkspaceTags")
	defer exit(err)

	addedOrUpdated, removed := compareMaps(latest.ko.Spec.Tags, desired.ko.Spec.Tags)

	if len(removed) > 0 {
		removeTags := []*string{}
		for i := range removed {
			removeTags = append(removeTags, &removed[i])
		}

		input := &svcsdk.UntagResourceInput{
			ResourceArn: (*string)(desired.ko.Status.ACKResourceMetadata.ARN),
			TagKeys:     aws.ToStringSlice(removeTags),
		}

		_, err = rm.sdkapi.UntagResource(ctx, input)
		rm.metrics.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}

	if len(addedOrUpdated) > 0 {
		input := &svcsdk.TagResourceInput{
			ResourceArn: (*string)(desired.ko.Status.ACKResourceMetadata.ARN),
			Tags:        aws.ToStringMap(addedOrUpdated),
		}
		_, err = rm.sdkapi.TagResource(ctx, input)
		rm.metrics.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
	}

	return nil

}

// updateWorkspaceAlias calls updateWorkspaceAlias to update a specific workspace
// alias.
func (rm *resourceManager) updateWorkspaceAlias(
	ctx context.Context,
	desired *resource,
) error {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.updateWorkspaceAlias")
	defer exit(err)

	input := &svcsdk.UpdateWorkspaceAliasInput{
		WorkspaceId: desired.ko.Status.WorkspaceID,
		Alias:       desired.ko.Spec.Alias,
	}

	_, err = rm.sdkapi.UpdateWorkspaceAlias(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UpdateWorkspaceAlias", err)
	if err != nil {
		return err
	}

	return nil
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
