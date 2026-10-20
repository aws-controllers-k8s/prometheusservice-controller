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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"

	svcapitypes "github.com/aws-controllers-k8s/prometheusservice-controller/apis/v1alpha1"
)

// scraperWithStatus builds a scraper resource whose reported state is the
// supplied status code. A nil code leaves the field unset, which is what the CR
// looks like before AWS has reported one.
func scraperWithStatus(statusCode *string) *resource {
	ko := &svcapitypes.Scraper{}
	ko.Status.StatusCode = statusCode
	return &resource{ko: ko}
}

func Test_scraperStatusCode(t *testing.T) {
	tests := []struct {
		name string
		r    *resource
		want string
	}{
		{"unset", scraperWithStatus(nil), ""},
		{"active", scraperWithStatus(aws.String("ACTIVE")), "ACTIVE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scraperStatusCode(tt.r); got != tt.want {
				t.Errorf("scraperStatusCode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func Test_scraperDeleting(t *testing.T) {
	tests := []struct {
		statusCode *string
		want       bool
	}{
		{nil, false},
		{aws.String(string(svcapitypes.ScraperStatusCode_DELETING)), true},
		{aws.String(string(svcapitypes.ScraperStatusCode_ACTIVE)), false},
		{aws.String(string(svcapitypes.ScraperStatusCode_CREATING)), false},
		{aws.String(string(svcapitypes.ScraperStatusCode_DELETION_FAILED)), false},
	}
	for _, tt := range tests {
		name := "nil"
		if tt.statusCode != nil {
			name = *tt.statusCode
		}
		t.Run(name, func(t *testing.T) {
			if got := scraperDeleting(scraperWithStatus(tt.statusCode)); got != tt.want {
				t.Errorf("scraperDeleting(%s) = %v, want %v", name, got, tt.want)
			}
		})
	}
}

func Test_scraperHasFailed(t *testing.T) {
	tests := []struct {
		statusCode *string
		want       bool
	}{
		{nil, false},
		{aws.String(string(svcapitypes.ScraperStatusCode_CREATION_FAILED)), true},
		{aws.String(string(svcapitypes.ScraperStatusCode_UPDATE_FAILED)), true},
		{aws.String(string(svcapitypes.ScraperStatusCode_DELETION_FAILED)), true},
		{aws.String(string(svcapitypes.ScraperStatusCode_ACTIVE)), false},
		{aws.String(string(svcapitypes.ScraperStatusCode_CREATING)), false},
		{aws.String(string(svcapitypes.ScraperStatusCode_UPDATING)), false},
		{aws.String(string(svcapitypes.ScraperStatusCode_DELETING)), false},
	}
	for _, tt := range tests {
		name := "nil"
		if tt.statusCode != nil {
			name = *tt.statusCode
		}
		t.Run(name, func(t *testing.T) {
			if got := scraperHasFailed(scraperWithStatus(tt.statusCode)); got != tt.want {
				t.Errorf("scraperHasFailed(%s) = %v, want %v", name, got, tt.want)
			}
		})
	}
}

func Test_compareMaps(t *testing.T) {
	tests := []struct {
		name               string
		a                  map[string]*string
		b                  map[string]*string
		wantAddedOrUpdated map[string]*string
		wantRemoved        []string
	}{
		{
			name:               "empty maps",
			a:                  map[string]*string{},
			b:                  map[string]*string{},
			wantAddedOrUpdated: map[string]*string{},
			wantRemoved:        nil,
		},
		{
			name:               "new element",
			a:                  map[string]*string{},
			b:                  map[string]*string{"k1": aws.String("v1")},
			wantAddedOrUpdated: map[string]*string{"k1": aws.String("v1")},
			wantRemoved:        nil,
		},
		{
			name:               "updated element",
			a:                  map[string]*string{"k1": aws.String("v1")},
			b:                  map[string]*string{"k1": aws.String("v2")},
			wantAddedOrUpdated: map[string]*string{"k1": aws.String("v2")},
			wantRemoved:        nil,
		},
		{
			name:               "removed element",
			a:                  map[string]*string{"k1": aws.String("v1")},
			b:                  map[string]*string{},
			wantAddedOrUpdated: map[string]*string{},
			wantRemoved:        []string{"k1"},
		},
		{
			name:               "added, updated and removed",
			a:                  map[string]*string{"k1": aws.String("v1"), "k2": aws.String("v2")},
			b:                  map[string]*string{"k2": aws.String("v2changed"), "k3": aws.String("v3")},
			wantAddedOrUpdated: map[string]*string{"k2": aws.String("v2changed"), "k3": aws.String("v3")},
			wantRemoved:        []string{"k1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddedOrUpdated, gotRemoved := compareMaps(tt.a, tt.b)
			if !reflect.DeepEqual(gotAddedOrUpdated, tt.wantAddedOrUpdated) {
				t.Errorf("compareMaps() addedOrUpdated = %v, want %v", gotAddedOrUpdated, tt.wantAddedOrUpdated)
			}
			if !reflect.DeepEqual(gotRemoved, tt.wantRemoved) {
				t.Errorf("compareMaps() removed = %v, want %v", gotRemoved, tt.wantRemoved)
			}
		})
	}
}

// scraperWithSecurityGroups builds a scraper resource whose EKS source carries
// the supplied security group IDs. A nil slice leaves the field unset, which is
// what the CR looks like when the user omits it from the manifest. Every other
// source field is held equal so a difference can only come from this one.
func scraperWithSecurityGroups(securityGroupIDs []string) *resource {
	ko := &svcapitypes.Scraper{}
	ko.Spec.Source = &svcapitypes.Source{
		EKSConfiguration: &svcapitypes.EKSConfiguration{
			ClusterARN: aws.String("arn:aws:eks:us-west-2:000000000000:cluster/test"),
			SubnetIDs:  aws.StringSlice([]string{"subnet-1", "subnet-2"}),
		},
	}
	if securityGroupIDs != nil {
		ko.Spec.Source.EKSConfiguration.SecurityGroupIDs = aws.StringSlice(securityGroupIDs)
	}
	return &resource{ko: ko}
}

// AMP assigns a security group when the user omits securityGroupIDs, so
// DescribeScraper reports one the spec never asked for. That must not register as
// a difference: Spec.Source is create-only and sdkUpdate rejects any difference
// beneath it as an immutable-source change, which would leave a scraper created
// without securityGroupIDs permanently unsynced. A security group the user did
// specify still has to be compared, so a real change is reported rather than
// silently ignored.
func Test_newResourceDelta_securityGroupIDs(t *testing.T) {
	const leafPath = "Spec.Source.EKSConfiguration.SecurityGroupIDs"
	tests := []struct {
		name          string
		desired       []string
		latest        []string
		wantDifferent bool
	}{
		{
			name:          "omitted by user, assigned by AMP",
			desired:       nil,
			latest:        []string{"sg-assigned"},
			wantDifferent: false,
		},
		{
			name:          "specified and unchanged",
			desired:       []string{"sg-1"},
			latest:        []string{"sg-1"},
			wantDifferent: false,
		},
		{
			name:          "specified, same set in a different order",
			desired:       []string{"sg-1", "sg-2"},
			latest:        []string{"sg-2", "sg-1"},
			wantDifferent: false,
		},
		{
			name:          "user changed the security group",
			desired:       []string{"sg-2"},
			latest:        []string{"sg-1"},
			wantDifferent: true,
		},
		{
			name:          "user added a security group",
			desired:       []string{"sg-1", "sg-2"},
			latest:        []string{"sg-1"},
			wantDifferent: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delta := newResourceDelta(
				scraperWithSecurityGroups(tt.desired),
				scraperWithSecurityGroups(tt.latest),
			)
			if got := delta.DifferentAt(leafPath); got != tt.wantDifferent {
				t.Errorf("delta.DifferentAt(%q) = %v, want %v", leafPath, got, tt.wantDifferent)
			}
			// sdkUpdate guards on the whole Spec.Source subtree, so the spurious
			// difference has to be absent there too, not just at the leaf.
			if got := delta.DifferentAt("Spec.Source"); got != tt.wantDifferent {
				t.Errorf("delta.DifferentAt(%q) = %v, want %v", "Spec.Source", got, tt.wantDifferent)
			}
		})
	}
}
