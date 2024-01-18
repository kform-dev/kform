package types

import (
	"context"
	"fmt"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	kformv1alpha1 "github.com/kform-dev/kform/apis/pkg/v1alpha1"
	"github.com/kform-dev/kform/pkg/data"
	"github.com/kform-dev/kform/pkg/util/cctx"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/yaml"
)

type Block interface {
	GetFileName() string
	GetPackageName() string
	GetBlockName() string
	GetBlockType() kformv1alpha1.BlockType
	GetContext(n string) string
	HasForEach() bool
	HasCount() bool
	GetSource() string
	GetProvider() string
	GetInputParameters() map[string]any
	GetProviders() map[string]string
	// Dependencies
	GetDependencies() sets.Set[string]
	GetPkgDependencies() sets.Set[string]
	UpdateDependencies(sets.Set[string])
	UpdatePkgDependencies(sets.Set[string])
	//GetKubeObject() *fn.KubeObject
	GetAttributes() *kformv1alpha1.Attributes
	GetData() *data.BlockData
	addData(ctx context.Context, ko *fn.KubeObject) error
}

func NewBlock(ctx context.Context, blockType kformv1alpha1.BlockType, blockName string, ko *fn.KubeObject) (*block, error) {
	d, err := getData(ctx, ko)
	if err != nil {
		return nil, err
	}
	blockData := data.NewBlockData()
	blockData.Add(data.DummyKey, d)

	return &block{
		blockType:   blockType,
		blockName:   blockName,
		fileNames:   []string{cctx.GetContextValue[string](ctx, CtxKeyFileName)},
		packageName: cctx.GetContextValue[string](ctx, CtxKeyPackageName),
		data:        blockData,
		attributes:  getAttributes(ctx, blockType, ko),
	}, nil
}

func getAttributes(ctx context.Context, blockType kformv1alpha1.BlockType, ko *fn.KubeObject) *kformv1alpha1.Attributes {
	sensitive := false
	if ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_SENSITIVE) != "" {
		sensitive = true
	}

	resourceType := ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_RESOURCE_TYPE)
	provider := "" // for non resources provider is irrelevant
	if blockType == kformv1alpha1.BlockTYPE_RESOURCE ||
		blockType == kformv1alpha1.BlockTYPE_DATA ||
		blockType == kformv1alpha1.BlockTYPE_LIST {
		provider = strings.Split(resourceType, "_")[0]
		if ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_PROVIDER) != "" {
			provider = ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_PROVIDER)
		}
	}

	return &kformv1alpha1.Attributes{
		APIVersion:    ko.GetAPIVersion(),
		Kind:          ko.GetKind(),
		ResourceType:  resourceType,
		ResourceID:    ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_RESOURCE_ID),
		Count:         ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_COUNT),
		ForEach:       ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_FOR_EACH),
		DependsOn:     ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_DEPENDS_ON),
		Provider:      provider,
		Description:   ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_DESCRIPTION),
		Sensitive:     sensitive,
		LifeCycle:     ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_LIFECYCLE),
		PreCondition:  ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_PRECONDITION),
		PostCondition: ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_POSTCONDITION),
		Provisioner:   ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_PROVISIONER),
		Organization:  ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_ORGANIZATION),
		Source:        ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_SOURCE), // TODO MIXIN
		Alias:         ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_ALIAS),
		HostName:      ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_HOSTNAME),

		// TODO
		//Validation:  ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_V),,
		//Providers: -> TBD maybe we need a dedicated KRM resource for Mixin
		//Source:        ko.GetAnnotation(kformv1alpha1.KformAnnotationKey_SOURCE), -> TBD maybe we need a dedicated KRM resource for Mixin
		// Workspaces
	}
}

type block struct {
	blockType       kformv1alpha1.BlockType
	blockName       string   // resource have a special syntax <reource-type>.<resource-id>, but the other have <block-type>-<unique-id>
	fileNames       []string // can be more then 1 for input, the rest will be 1
	packageName     string
	data            *data.BlockData
	attributes      *kformv1alpha1.Attributes
	dependencies    sets.Set[string]
	pkgDependencies sets.Set[string]
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

func (r *block) GetData() *data.BlockData { return r.data }

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

func (r *block) addData(ctx context.Context, ko *fn.KubeObject) error {
	d, err := getData(ctx, ko)
	if err != nil {
		return err
	}
	r.data.Add(data.DummyKey, d)
	return nil
}

func getData(ctx context.Context, ko *fn.KubeObject) (any, error) {
	if ko == nil {
		return nil, fmt.Errorf("cannot create a block without a kubeobject")
	}
	var v any
	if err := yaml.Unmarshal([]byte(ko.String()), &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal the kubeobject, err: %s", err.Error())
	}
	return v, nil
}
