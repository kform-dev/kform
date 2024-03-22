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
	"encoding/json"

	"github.com/kform-dev/kform/pkg/util/cctx"
)

type CtxKey string

func (c CtxKey) String() string {
	return "context key " + string(c)
}

const (
	CtxKeyRecorder     CtxKey = "recorder"
	CtxKeyPackage      CtxKey = "package" // store the Package
	CtxKeyPackageName  CtxKey = "packageName"
	CtxKeyPackageKind  CtxKey = "packageKind"
	CtxKeyFileName     CtxKey = "fileName"
	CtxKeyIndex        CtxKey = "index"
	CtxKeyBlockType    CtxKey = "blockType"
	CtxKeyResourceID   CtxKey = "resourceID"
	CtxKeyResourceType CtxKey = "resourcetype"
	CtxKeyYamlRNODE    CtxKey = "yamlRNODE" // stores the kubeObject
)

type PackageKind int

const (
	PackageKind_ROOT PackageKind = iota
	PackageKind_MIXIN
)

func (d PackageKind) String() string {
	return [...]string{"root", "mixin"}[d]
}

type ContextAPI struct {
	FileName     string      `json:"fileName"`
	PackageKind  PackageKind `json:"packageKind"`
	PackageName  string      `json:"packageName"`
	BlockType    *string     `json:"blockType,omitempty"`
	ResourceID   *string     `json:"resourceID,omitempty"`
	ResourceType *string     `json:"resourceType,omitempty"`
}

type Context struct {
	context.Context
}

func (r Context) String() string {
	c := ContextAPI{}
	blockType := cctx.GetContextValue[string](r.Context, CtxKeyBlockType)
	if blockType != "" {
		c.BlockType = &blockType
	}
	resourceID := cctx.GetContextValue[string](r.Context, CtxKeyResourceID)
	if resourceID != "" {
		c.ResourceID = &resourceID
	}
	resourceType := cctx.GetContextValue[string](r.Context, CtxKeyResourceType)
	if resourceType != "" {
		c.ResourceType = &resourceType
	}
	c.PackageName = cctx.GetContextValue[string](r.Context, CtxKeyPackageName)
	c.PackageKind = cctx.GetContextValue[PackageKind](r.Context, CtxKeyPackageKind)
	c.FileName = cctx.GetContextValue[string](r.Context, CtxKeyFileName)

	b, err := json.Marshal(c)
	if err != nil {
		return ""
	}
	return string(b)
}
