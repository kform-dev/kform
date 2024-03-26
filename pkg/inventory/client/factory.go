package client

import (
	"github.com/kform-dev/kform/pkg/inventory/policy"
	"k8s.io/kubectl/pkg/cmd/util"
)

var (
	_ ClientFactory = ClusterClientFactory{}
)

// ClientFactory is a factory that constructs new Client instances.
type ClientFactory interface {
	NewClient(factory util.Factory) (Client, error)
}

// ClusterClientFactory is a factory that creates instances of ClusterClient inventory client.
type ClusterClientFactory struct {
	StatusPolicy policy.StatusPolicy
}

func (r ClusterClientFactory) NewClient(factory util.Factory) (Client, error) {
	return NewClient(factory, WrapInventoryObj, InvInfoToConfigMap, r.StatusPolicy)
}
