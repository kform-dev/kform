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

package types

import (
	"context"
	"fmt"
	"strings"

	"github.com/henderiw/logger/log"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/util/cctx"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Block interface {
	GetFileName() string
	GetIndex() string
	GetPackageName() string
	GetBlockName() string
	GetBlockType() kformv1alpha1.BlockType
	GetContext(n string) string
	HasForEach() bool
	HasCount() bool
	GetSource() string
	GetProvider() string
	GetInputParameters() map[string]any
	GetProviders() map[string]string // only relevant for mixin
	// Dependencies
	GetDependencies() sets.Set[string]
	GetPkgDependencies() sets.Set[string]
	UpdateDependencies(sets.Set[string])
	UpdatePkgDependencies(sets.Set[string])
	//GetKubeObject() *fn.KubeObject
	GetAttributes() *kformv1alpha1.Attributes
	GetData() data.BlockData
	addData(ctx context.Context, rn *yaml.RNode) error
}

func NewBlock(ctx context.Context, blockType kformv1alpha1.BlockType, blockName string, rn *yaml.RNode) (*block, error) {
	blockData := data.BlockData{} // initialize
	blockData = blockData.Add(rn)

	return &block{
		blockType:   blockType,
		blockName:   blockName,
		fileNames:   []string{cctx.GetContextValue[string](ctx, CtxKeyFileName)},
		index:       cctx.GetContextValue[string](ctx, CtxKeyIndex),
		packageName: cctx.GetContextValue[string](ctx, CtxKeyPackageName),
		data:        blockData,
		attributes:  getAttributes(ctx, blockType, rn),
	}, nil
}

type block struct {
	blockType       kformv1alpha1.BlockType
	blockName       string   // resource have a special syntax <reource-type>.<resource-id>, but the other have <block-type>.<unique-id>
	fileNames       []string // can be more then 1 for input, the rest will be 1
	index           string   // index is the index entry within the fileName if multiple yaml docs were put in a single filename
	packageName     string
	data            data.BlockData
	attributes      *kformv1alpha1.Attributes
	dependencies    sets.Set[string] // dependencies within the package
	pkgDependencies sets.Set[string] // dependencies to other packages
}

func (r *block) GetBlockName() string { return r.blockName }

func (r *block) GetPackageName() string { return r.packageName }

func (r *block) GetBlockType() kformv1alpha1.BlockType { return r.blockType }

func (r *block) GetDependencies() sets.Set[string] { return r.dependencies }

func (r *block) UpdateDependencies(d sets.Set[string]) { r.dependencies = d }

func (r *block) GetPkgDependencies() sets.Set[string] { return r.pkgDependencies }

func (r *block) UpdatePkgDependencies(d sets.Set[string]) { r.pkgDependencies = d }

func (r *block) HasForEach() bool { return r.attributes.ForEach != "" }

func (r *block) HasCount() bool { return r.attributes.Count != "" }

func (r *block) GetContext(n string) string {
	return getContext(r.GetFileName(), r.packageName, n, r.blockType)
}

func (r *block) GetSource() string { return r.attributes.Source }

func (r *block) GetProvider() string { return r.attributes.Provider }

func (r *block) GetInputParameters() map[string]any { return r.attributes.InputParameters }

func (r *block) GetProviders() map[string]string { return r.attributes.Providers } // Mixin

func (r *block) GetData() data.BlockData { return r.data }

func (r *block) GetAttributes() *kformv1alpha1.Attributes { return r.attributes }

func getContext(fileName string, packageName, blockName string, blockType kformv1alpha1.BlockType) string {
	// fileName is a parsed fileName
	return fmt.Sprintf("fileName=%s, moduleName=%s name=%s, blockType=%s", fileName, packageName, blockName, blockType.String())
}

func (r *block) GetFileName() string {
	var sb strings.Builder
	for i, fileName := range r.fileNames {
		if i == 0 {
			sb.WriteString(fileName)
		} else {
			sb.WriteString(fmt.Sprintf(",%s", fileName))
		}
	}
	return sb.String()
}

func (r *block) GetIndex() string {
	return r.index
}

func (r *block) addData(ctx context.Context, rn *yaml.RNode) error {
	if rn == nil {
		return fmt.Errorf("cannot create a block without a yaml.RNode")
	}
	/*
		d, err := getData(ctx, rn)
		if err != nil {
			return err
		}
	*/
	r.data = r.data.Add(rn)
	return nil
}

/*
func getData(ctx context.Context, rn *yaml.RNode) (any, error) {
	log := log.FromContext(ctx)
	log.Debug("getData")
	if rn == nil {
		return nil, fmt.Errorf("cannot create a block without a yaml.RNode")
	}
	var v map[string]any
	if err := yaml.Unmarshal([]byte(ko.String()), &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal the kubeobject, err: %s", err.Error())
	}
	return v, nil
}
*/

func getAttributes(ctx context.Context, blockType kformv1alpha1.BlockType, rn *yaml.RNode) *kformv1alpha1.Attributes {
	log := log.FromContext(ctx)
	log.Debug("getAttributes")

	annotations := rn.GetAnnotations()
	sensitive := false
	if annotations[kformv1alpha1.KformAnnotationKey_SENSITIVE] != "" {
		sensitive = true
	}

	resourceType := annotations[kformv1alpha1.KformAnnotationKey_RESOURCE_TYPE]
	provider := "" // for non resources provider is irrelevant
	if blockType == kformv1alpha1.BlockTYPE_RESOURCE ||
		blockType == kformv1alpha1.BlockTYPE_DATA ||
		blockType == kformv1alpha1.BlockTYPE_LIST {
		provider = strings.SplitN(resourceType, "_", 2)[0]
		if annotations[kformv1alpha1.KformAnnotationKey_PROVIDER] != "" {
			provider = annotations[kformv1alpha1.KformAnnotationKey_PROVIDER]
		}
	}

	return &kformv1alpha1.Attributes{
		APIVersion:    rn.GetApiVersion(),
		Kind:          rn.GetKind(),
		ResourceType:  resourceType,
		ResourceID:    annotations[kformv1alpha1.KformAnnotationKey_RESOURCE_ID],
		Count:         annotations[kformv1alpha1.KformAnnotationKey_COUNT],
		ForEach:       annotations[kformv1alpha1.KformAnnotationKey_FOR_EACH],
		DependsOn:     annotations[kformv1alpha1.KformAnnotationKey_DEPENDS_ON],
		Provider:      provider,
		Description:   annotations[kformv1alpha1.KformAnnotationKey_DESCRIPTION],
		Sensitive:     sensitive,
		LifeCycle:     annotations[kformv1alpha1.KformAnnotationKey_LIFECYCLE],
		PreCondition:  annotations[kformv1alpha1.KformAnnotationKey_PRECONDITION],
		PostCondition: annotations[kformv1alpha1.KformAnnotationKey_POSTCONDITION],
		Provisioner:   annotations[kformv1alpha1.KformAnnotationKey_PROVISIONER],
		Organization:  annotations[kformv1alpha1.KformAnnotationKey_ORGANIZATION],
		Source:        annotations[kformv1alpha1.KformAnnotationKey_SOURCE], // TODO MIXIN
		Alias:         annotations[kformv1alpha1.KformAnnotationKey_ALIAS],
		HostName:      annotations[kformv1alpha1.KformAnnotationKey_HOSTNAME],

		// TODO
		//Validation:  ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_V),,
		//Providers: -> TBD maybe we need a dedicated KRM resource for Mixin
		//Source:        ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_SOURCE), -> TBD maybe we need a dedicated KRM resource for Mixin
		// Workspaces
	}
}
