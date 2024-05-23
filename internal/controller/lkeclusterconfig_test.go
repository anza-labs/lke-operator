/*
Copyright 2024 lke-operator contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"errors"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/anza-labs/lke-operator/api/v1alpha1"
	internalerrors "github.com/anza-labs/lke-operator/internal/errors"
	"github.com/linode/linodego"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_extractTags(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		rawTags      string
		expectedTags []string
	}{
		"single_tag": {"foo", []string{"foo"}},
		"csv":        {"foo,bar,baz", []string{"foo", "bar", "baz"}},
		"lf":         {"foo\nbar\nbaz", []string{"foo", "bar", "baz"}},
		"cr":         {"foo\rbar\rbaz", []string{"foo", "bar", "baz"}},
		"crlf":       {"foo\r\nbar\r\nbaz", []string{"foo", "bar", "baz"}},
		"mixed":      {"foo\r\nbar,baz", []string{"foo", "bar", "baz"}},
		"empty":      {"", []string{}},
		"empty_csv":  {",,,,", []string{}},
		"empty_cr":   {"\r\r", []string{}},
		"empty_lf":   {"\n\n", []string{}},
		"empty_crlf": {"\r\n", []string{}},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			tags := extractTags(tc.rawTags)
			if !slices.Equal(tags, tc.expectedTags) {
				t.Errorf("expected Tags value: %#+v, got: %#+v",
					tc.expectedTags, tags)
			}
		})
	}
}

func Test_updateTags(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lke          *v1alpha1.LKEClusterConfig
		cluster      *linodego.LKECluster
		expectedOpts linodego.LKEClusterUpdateOptions
	}{
		"noop": {
			lke: &v1alpha1.LKEClusterConfig{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				lkeTagsAnnotation: "foo",
			}}},
			cluster: &linodego.LKECluster{
				Tags: []string{"foo"},
			},
			expectedOpts: linodego.LKEClusterUpdateOptions{},
		},
		"replace": {
			lke: &v1alpha1.LKEClusterConfig{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				lkeTagsAnnotation: "foo",
			}}},
			cluster: &linodego.LKECluster{
				Tags: []string{"bar"},
			},
			expectedOpts: linodego.LKEClusterUpdateOptions{Tags: mkptr([]string{"foo"})},
		},
		"replace_multiple": {
			lke: &v1alpha1.LKEClusterConfig{ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{
				lkeTagsAnnotation: "foo,bar",
			}}},
			cluster: &linodego.LKECluster{
				Tags: []string{"baz"},
			},
			expectedOpts: linodego.LKEClusterUpdateOptions{Tags: mkptr([]string{"bar", "foo"})},
		},
		"empty": {
			lke: &v1alpha1.LKEClusterConfig{ObjectMeta: v1.ObjectMeta{}},
			cluster: &linodego.LKECluster{
				Tags: []string{"baz"},
			},
			expectedOpts: linodego.LKEClusterUpdateOptions{},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := linodego.LKEClusterUpdateOptions{}

			opts = updateTags(tc.lke, tc.cluster, opts)

			if tc.expectedOpts.Tags == nil {
				if tc.expectedOpts.Tags != opts.Tags {
					t.Errorf("expected Tags value: %#+v, got: %#+v",
						tc.expectedOpts.Tags, opts.Tags)
				}
				return
			}

			if !slices.Equal(*opts.Tags, *tc.expectedOpts.Tags) {
				t.Errorf("expected Tags value: %#+v, got: %#+v",
					*tc.expectedOpts.Tags, *opts.Tags)
			}
		})
	}
}

func Test_updateControlPlane(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lke            *v1alpha1.LKEClusterConfig
		cluster        *linodego.LKECluster
		expectedHA     bool
		expectedChange bool
	}{
		"sa-no-change": {
			lke: &v1alpha1.LKEClusterConfig{Spec: v1alpha1.LKEClusterConfigSpec{
				HighAvailability: mkptr(false),
			}},
			cluster: &linodego.LKECluster{
				ControlPlane: linodego.LKEClusterControlPlane{HighAvailability: false},
			},
			expectedHA:     false,
			expectedChange: false,
		},
		"ha-change": {
			lke: &v1alpha1.LKEClusterConfig{Spec: v1alpha1.LKEClusterConfigSpec{
				HighAvailability: mkptr(true),
			}},
			cluster: &linodego.LKECluster{
				ControlPlane: linodego.LKEClusterControlPlane{HighAvailability: false},
			},
			expectedHA:     true,
			expectedChange: true,
		},
		"sa-change": {
			lke: &v1alpha1.LKEClusterConfig{Spec: v1alpha1.LKEClusterConfigSpec{
				HighAvailability: mkptr(false),
			}},
			cluster: &linodego.LKECluster{
				ControlPlane: linodego.LKEClusterControlPlane{HighAvailability: true},
			},
			expectedHA:     false,
			expectedChange: true,
		},
		"ha-no-change": {
			lke: &v1alpha1.LKEClusterConfig{Spec: v1alpha1.LKEClusterConfigSpec{
				HighAvailability: mkptr(true),
			}},
			cluster: &linodego.LKECluster{
				ControlPlane: linodego.LKEClusterControlPlane{HighAvailability: true},
			},
			expectedHA:     true,
			expectedChange: false,
		},
		"empty": {
			lke: &v1alpha1.LKEClusterConfig{Spec: v1alpha1.LKEClusterConfigSpec{}},
			cluster: &linodego.LKECluster{
				ControlPlane: linodego.LKEClusterControlPlane{HighAvailability: false},
			},
			expectedHA:     false,
			expectedChange: false,
		},
	} {
		tc := tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := linodego.LKEClusterUpdateOptions{}

			opts, change := updateControlPlane(tc.lke, tc.cluster, opts)
			if change != tc.expectedChange {
				t.Errorf("expected Change value: %#+v, got: %#+v",
					tc.expectedChange, change)
			}

			if opts.ControlPlane.HighAvailability == nil || *opts.ControlPlane.HighAvailability != tc.expectedHA {
				t.Errorf("expected HA value: %#+v, got: %#+v",
					tc.expectedHA, opts.ControlPlane.HighAvailability)
			}
		})
	}
}

func Test_getMajorMinor(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input         string
		expectedMajor int
		expectedMinor int
		targetError   error
	}{
		{"1.28", 1, 28, nil},
		{"1.9999999", 1, 9999999, nil},
		{"0.28", 0, 28, nil},
		{"0.0", 0, 0, nil},
		{"0.28+lke", -1, -1, strconv.ErrSyntax},
		{"-1.28", -1, -1, internalerrors.ErrInvalidLKEVersion},
		{"1.28.1", -1, -1, internalerrors.ErrInvalidLKEVersion},
	} {
		tc := tc

		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			major, minor, err := getMajorMinor(tc.input)
			if !errors.Is(err, tc.targetError) {
				t.Errorf("expected Error value: %#+v, got: %#+v",
					tc.targetError, err)
			}

			if major != tc.expectedMajor {
				t.Errorf("expected Major value: %#+v, got: %#+v",
					tc.expectedMajor, major)
			}

			if minor != tc.expectedMinor {
				t.Errorf("expected Minor value: %#+v, got: %#+v",
					tc.expectedMinor, minor)
			}
		})
	}
}

func Test_makeNodePools(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lkenp        map[string]v1alpha1.LKENodePool
		expectedOpts []linodego.LKENodePoolCreateOptions
	}{
		"": {
			lkenp: map[string]v1alpha1.LKENodePool{
				"foo":   {NodeCount: 1, LinodeType: "g6-standard-1"},
				"bar.1": {NodeCount: 2, LinodeType: "g6-standard-1"},
				"bar.2": {NodeCount: 2, LinodeType: "g6-standard-1"},
				"baz": {NodeCount: 3, LinodeType: "g6-standard-1", Autoscaler: &v1alpha1.LKENodePoolAutoscaler{
					Min: 1,
					Max: 5,
				}},
			},
			expectedOpts: []linodego.LKENodePoolCreateOptions{
				{Count: 1, Type: "g6-standard-1"},
				{Count: 2, Type: "g6-standard-1"},
				{Count: 2, Type: "g6-standard-1"},
				{Count: 3, Type: "g6-standard-1", Autoscaler: &linodego.LKENodePoolAutoscaler{
					Min: 1,
					Max: 5,
				}},
			},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			opts := makeNodePools(tc.lkenp)

			compareFunc := func(a, b linodego.LKENodePoolCreateOptions) int {
				switch {
				case a.Count > b.Count:
					return 1
				case a.Count < b.Count:
					return -1
				}

				if a.Autoscaler == nil {
					if a.Autoscaler != b.Autoscaler {
						return -1
					}
				} else {
					switch {
					case a.Autoscaler.Max > b.Autoscaler.Max:
						return 1
					case a.Autoscaler.Max < b.Autoscaler.Max:
						return -1
					case a.Autoscaler.Min > b.Autoscaler.Min:
						return 1
					case a.Autoscaler.Min < b.Autoscaler.Min:
						return -1
					}
				}

				return strings.Compare(a.Type, b.Type)
			}

			slices.SortFunc(opts, compareFunc)
			slices.SortFunc(tc.expectedOpts, compareFunc)

			if slices.CompareFunc(tc.expectedOpts, opts, compareFunc) != 0 {
				t.Errorf("expected Opts value: %#+v, got: %#+v",
					tc.expectedOpts, opts)
			}
		})
	}
}

func Test_getLatestVersion(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		versions       []linodego.LKEVersion
		expectedLatest linodego.LKEVersion
		targetError    error
	}{
		"single": {
			versions: []linodego.LKEVersion{
				{ID: "1.28"},
			},
			expectedLatest: linodego.LKEVersion{ID: "1.28"},
			targetError:    nil,
		},
		"multiple": {
			versions: []linodego.LKEVersion{
				{ID: "1.28"},
				{ID: "1.29"},
				{ID: "1.30"},
			},
			expectedLatest: linodego.LKEVersion{ID: "1.30"},
			targetError:    nil,
		},
		"empty": {
			versions:       []linodego.LKEVersion{},
			expectedLatest: linodego.LKEVersion{},
			targetError:    internalerrors.ErrInvalidLKEVersion,
		},
		"invalid": {
			versions: []linodego.LKEVersion{
				{ID: "1.30-lke"},
			},
			expectedLatest: linodego.LKEVersion{},
			targetError:    strconv.ErrSyntax,
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			latest, err := getLatestVersion(tc.versions)
			if !errors.Is(err, tc.targetError) {
				t.Errorf("expected Error value: %#+v, got: %#+v",
					tc.targetError, err)
			}

			if latest.ID != tc.expectedLatest.ID {
				t.Errorf("expected Latest value: %#+v, got: %#+v",
					tc.expectedLatest, latest)
			}
		})
	}
}

func Test_generateNodePoolStatusesFromSpec(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lkeNP       map[string]v1alpha1.LKENodePool
		expectedNPS map[string]v1alpha1.NodePoolStatus
	}{
		"empty": {
			lkeNP:       map[string]v1alpha1.LKENodePool{},
			expectedNPS: map[string]v1alpha1.NodePoolStatus{},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			nps := generateNodePoolStatusesFromSpec(tc.lkeNP)
			if !reflect.DeepEqual(nps, tc.expectedNPS) {
				t.Errorf("expected NodePoolStatuses value: %#+v, got: %#+v",
					tc.expectedNPS, nps)
			}
		})
	}
}

func Test_generateNodePoolStatusesFromAPI(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		lkeNP       []linodego.LKENodePool
		expectedNPS map[string]v1alpha1.NodePoolStatus
	}{
		"empty": {
			lkeNP:       []linodego.LKENodePool{},
			expectedNPS: map[string]v1alpha1.NodePoolStatus{},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			nps := generateNodePoolStatusesFromAPI(tc.lkeNP)
			if !reflect.DeepEqual(nps, tc.expectedNPS) {
				t.Errorf("expected NodePoolStatuses value: %#+v, got: %#+v",
					tc.expectedNPS, nps)
			}
		})
	}
}

func Test_compareNodePoolStatuses(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		nps1, nps2                                     map[string]v1alpha1.NodePoolStatus
		expectedCreate, expectedUpdate, expectedDelete map[string]v1alpha1.NodePoolStatus
	}{
		"empty": {
			nps1:           map[string]v1alpha1.NodePoolStatus{},
			nps2:           map[string]v1alpha1.NodePoolStatus{},
			expectedCreate: map[string]v1alpha1.NodePoolStatus{},
			expectedUpdate: map[string]v1alpha1.NodePoolStatus{},
			expectedDelete: map[string]v1alpha1.NodePoolStatus{},
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			create, update, delete := compareNodePoolStatuses(tc.nps1, tc.nps2)

			if !reflect.DeepEqual(create, tc.expectedCreate) {
				t.Errorf("expected Create value: %#+v, got: %#+v",
					tc.expectedCreate, create)
			}

			if !reflect.DeepEqual(update, tc.expectedUpdate) {
				t.Errorf("expected Update value: %#+v, got: %#+v",
					tc.expectedUpdate, update)
			}

			if !reflect.DeepEqual(delete, tc.expectedDelete) {
				t.Errorf("expected Delete value: %#+v, got: %#+v",
					tc.expectedDelete, delete)
			}
		})
	}
}
