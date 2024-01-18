package types

import (
	kfplugin "github.com/henderiw-nephio/kform/kform-plugin/plugin"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Provider struct {
	Name string
	//ExecPath string
	Initializer
	Resources       sets.Set[string]
	ReadDataSources sets.Set[string]
	ListDataSources sets.Set[string]
}

type Initializer func() (kfplugin.Provider, error)
