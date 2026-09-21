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
