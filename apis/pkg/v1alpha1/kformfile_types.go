package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KformFile struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec KformFileSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

type KformFileSpec struct {
	// +kubebuilder:validation:Enum=provider;module
	ProviderRequirements map[string]Provider `json:"providerRequirements" yaml:"providerRequirements"`
	Info                 Info                `json:"info,omitempty" yaml:"info,omitempty"`
}

type Provider struct {
	Source  string `json:"source" yaml:"source"`
	Version string `json:"version" yaml:"version"`
}

type Info struct {
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Maintainers []Maintainer `json:"maintainers,omitempty" yaml:"maintainers,omitempty"`
}

type Maintainer struct {
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
}

var (
	KformFileKind = reflect.TypeOf(KformFile{}).Name()
)
