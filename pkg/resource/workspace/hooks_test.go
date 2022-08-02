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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
)

// Test function obtained from:
// https://github.com/aws-controllers-k8s/lambda-controller/blob/7a525a074d7ee220eddbb66b584adec4299a2704/pkg/resource/function/hooks_test.go
func Test_compareMaps(t *testing.T) {
	type args struct {
		a map[string]*string
		b map[string]*string
	}
	tests := []struct {
		name               string
		args               args
		wantAddedorUpdated map[string]*string
		wantRemoved        []string
	}{
		{
			name: "empty maps",
			args: args{
				a: map[string]*string{},
				b: map[string]*string{},
			},
			wantAddedorUpdated: map[string]*string{},
			wantRemoved:        nil,
		},
		{
			name: "new elements",
			args: args{
				a: map[string]*string{},
				b: map[string]*string{"k1": aws.String("v1")},
			},
			wantAddedorUpdated: map[string]*string{"k1": aws.String("v1")},
			wantRemoved:        nil,
		},
		{
			name: "updated elements",
			args: args{
				a: map[string]*string{"k1": aws.String("v1"), "k2": aws.String("v2")},
				b: map[string]*string{"k1": aws.String("v10"), "k2": aws.String("v20")},
			},
			wantAddedorUpdated: map[string]*string{"k1": aws.String("v10"), "k2": aws.String("v20")},
			wantRemoved:        nil,
		},
		{
			name: "removed elements",
			args: args{
				a: map[string]*string{"k1": aws.String("v1"), "k2": aws.String("v2")},
				b: map[string]*string{"k1": aws.String("v1")},
			},
			wantAddedorUpdated: map[string]*string{},
			wantRemoved:        []string{"k2"},
		},
		{
			name: "added, updated and removed elements",
			args: args{
				a: map[string]*string{"k1": aws.String("v1"), "k2": aws.String("v2")},
				b: map[string]*string{"k1": aws.String("v10"), "k3": aws.String("v3")},
			},
			wantAddedorUpdated: map[string]*string{"k3": aws.String("v3"), "k1": aws.String("v10")},
			wantRemoved:        []string{"k2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddedOrUpdated, gotRemoved := compareMaps(tt.args.a, tt.args.b)
			if !reflect.DeepEqual(gotAddedOrUpdated, tt.wantAddedorUpdated) {
				t.Errorf("compareMaps() gotAddedOrUpdated = %v, want %v", gotAddedOrUpdated, tt.wantAddedorUpdated)
			}
			if !reflect.DeepEqual(gotRemoved, tt.wantRemoved) {
				t.Errorf("compareMaps() gotRemoved = %v, want %v", gotRemoved, tt.wantRemoved)
			}
		})
	}
}
