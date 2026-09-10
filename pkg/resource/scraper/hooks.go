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

package scraper

import (
	"context"

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/amp"

	svcapitypes "github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

// scraperStatusCode returns the scraper's current state, or the empty string
// when AWS has not reported one yet.
func scraperStatusCode(r *resource) string {
	if r.ko.Status.Status == nil || r.ko.Status.Status.StatusCode == nil {
		return ""
	}
	return *r.ko.Status.Status.StatusCode
}

// scraperInTransition returns true while AWS is still converging the scraper.
func scraperInTransition(r *resource) bool {
	switch scraperStatusCode(r) {
	case string(svcapitypes.ScraperStatusCode_CREATING),
		string(svcapitypes.ScraperStatusCode_UPDATING),
		string(svcapitypes.ScraperStatusCode_DELETING):
		return true
	}
	return false
}

// scraperHasFailed returns true when the scraper reached a state AWS will not
// retry, so the resource should be marked terminal rather than requeued.
func scraperHasFailed(r *resource) bool {
	switch scraperStatusCode(r) {
	case string(svcapitypes.ScraperStatusCode_CREATION_FAILED),
		string(svcapitypes.ScraperStatusCode_UPDATE_FAILED),
		string(svcapitypes.ScraperStatusCode_DELETION_FAILED):
		return true
	}
	return false
}

// syncTags reconciles scraper tags with the separate TagResource/UntagResource
// APIs, since UpdateScraper accepts no tags. It is invoked by the generated
// sdkUpdate because Spec.Tags is configured with custom_sync.
func (rm *resourceManager) syncTags(
	ctx context.Context,
	desired *resource,
	latest *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.syncTags")
	defer func() { exit(err) }()

	if latest.ko.Status.ACKResourceMetadata == nil ||
		latest.ko.Status.ACKResourceMetadata.ARN == nil {
		return nil
	}
	resourceARN := (*string)(latest.ko.Status.ACKResourceMetadata.ARN)

	addedOrUpdated, removed := compareMaps(latest.ko.Spec.Tags, desired.ko.Spec.Tags)

	if len(removed) > 0 {
		_, err = rm.sdkapi.UntagResource(ctx, &svcsdk.UntagResourceInput{
			ResourceArn: resourceARN,
			TagKeys:     removed,
		})
		rm.metrics.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}

	if len(addedOrUpdated) > 0 {
		_, err = rm.sdkapi.TagResource(ctx, &svcsdk.TagResourceInput{
			ResourceArn: resourceARN,
			Tags:        aws.ToStringMap(addedOrUpdated),
		})
		rm.metrics.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
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
	for keyA, valueA := range a {
		valueB, found := b[keyA]
		if !found {
			removed = append(removed, keyA)
			continue
		}
		if *valueA != *valueB {
			addedOrUpdated[keyA] = valueB
		}
	}
	for keyB, valueB := range b {
		if _, found := a[keyB]; !found {
			addedOrUpdated[keyB] = valueB
		}
	}
	return
}
