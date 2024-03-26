/*
Copyright 2024 Nokia.

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

package pkgio

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/henderiw/store"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func TestKformStreamReader(t *testing.T) {
	cases := map[string]struct {
		path           string
		input          bool
		expectedOutput map[types.NamespacedName][][]string
	}{
		"SingleDoc_Without_Annotations": {
			path: "testdocs/doc1.yaml",
			expectedOutput: map[types.NamespacedName][][]string{
				{Namespace: "0", Name: "stream"}: {
					//{KformInternalPkgioAnnotationInternalKey_Index, "0"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc1.yaml"},
				},
			},
		},
		"SingleDoc_Without_Annotations_AsInput": {
			path:  "testdocs/doc1.yaml",
			input: true,
			expectedOutput: map[types.NamespacedName][][]string{
				{Namespace: "0", Name: "stream"}: {
					//{KformInternalPkgioAnnotationInternalKey_Index, "0"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc1.yaml"},
					{kformv1alpha1.KformAnnotationKey_BLOCK_TYPE, kformv1alpha1.BlockTYPE_INPUT.String()},
				},
			},
		},
		"SingleDoc_With_Annotations": {
			path: "testdocs/doc2.yaml",
			expectedOutput: map[types.NamespacedName][][]string{
				{Namespace: "0", Name: "stream"}: {
					{"a", "b"},
					//{KformInternalPkgioAnnotationInternalKey_Index, "0"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc2.yaml"},
				},
			},
		},
		"SingleDoc_With_Annotations_AsInput": {
			path:  "testdocs/doc2.yaml",
			input: true,
			expectedOutput: map[types.NamespacedName][][]string{
				{Namespace: "0", Name: "stream"}: {
					{"a", "b"},
					//{KformInternalPkgioAnnotationInternalKey_Index, "0"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc2.yaml"},
					{kformv1alpha1.KformAnnotationKey_BLOCK_TYPE, kformv1alpha1.BlockTYPE_INPUT.String()},
				},
			},
		},
		"DualDoc_With_Annotations": {
			path: "testdocs/doc3.yaml",
			expectedOutput: map[types.NamespacedName][][]string{
				{Namespace: "0", Name: "stream"}: {
					{"a", "b"},
					//{KformInternalPkgioAnnotationInternalKey_Index, "0"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc3.yaml"},
				},
				{Namespace: "1", Name: "stream"}: {
					{"c", "d"},
					//{KformInternalPkgioAnnotationInternalKey_Index, "1"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc3.yaml"},
				},
			},
		},
		"DualDoc_With_Annotations_AsInput": {
			path:  "testdocs/doc3.yaml",
			input: true,
			expectedOutput: map[types.NamespacedName][][]string{
				{Namespace: "0", Name: "stream"}: {
					{"a", "b"},
					//{KformInternalPkgioAnnotationInternalKey_Index, "0"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc3.yaml"},
					{kformv1alpha1.KformAnnotationKey_BLOCK_TYPE, kformv1alpha1.BlockTYPE_INPUT.String()},
				},
				{Namespace: "1", Name: "stream"}: {
					{"c", "d"},
					//{KformInternalPkgioAnnotationInternalKey_Index, "1"},
					//{KformInternalPkgioAnnotationInternalKey_Path, "testdocs/doc3.yaml"},
					{kformv1alpha1.KformAnnotationKey_BLOCK_TYPE, kformv1alpha1.BlockTYPE_INPUT.String()},
				},
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()

			f, err := os.Open(tc.path)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
			}
			defer f.Close()

			r := KformStreamReader{
				Reader: f,
				Input:  tc.input,
			}
			datastore, err := r.Read(ctx)
			if err != nil {
				t.Errorf("unexpected error: %s", err.Error())
			}

			output := map[types.NamespacedName][][]string{}
			datastore.List(ctx, func(ctx context.Context, key store.Key, rn *yaml.RNode) {
				output[key.NamespacedName] = [][]string{}
				for k, v := range rn.GetAnnotations() {
					k := k
					v := v
					output[key.NamespacedName] = append(output[key.NamespacedName], []string{k, v})
				}
			})
			for expectedNSN, expectedAnnotations := range tc.expectedOutput {
				annotations, ok := output[expectedNSN]
				if !ok {
					t.Errorf("expected output not present: %s", expectedAnnotations)
					continue
				}
				sort.SliceStable(annotations, func(i, j int) bool {
					return annotations[i][0] < annotations[j][0]
				})

				if diff := cmp.Diff(annotations, expectedAnnotations); diff != "" {
					t.Errorf("want %s, got: %s", expectedAnnotations, annotations)
				}
				delete(output, expectedNSN)

			}
			if len(output) != 0 {
				t.Errorf("unexpected output got %v", output)
			}
		})
	}
}
