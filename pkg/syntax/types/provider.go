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
	"os/exec"

	"github.com/henderiw/logger/log"
	"github.com/kform-dev/kform-plugin/kfprotov1/kfplugin1"
	kfplugin "github.com/kform-dev/kform-plugin/plugin"
	"github.com/kform-dev/kform/pkg/syntax/types/logging"
	"github.com/kform-dev/plugin"
	"k8s.io/apimachinery/pkg/util/sets"
)

type Provider struct {
	Name string
	Initializer
	Resources       sets.Set[string]
	ReadDataSources sets.Set[string]
	ListDataSources sets.Set[string]
}

type Initializer func() (kfplugin.Provider, error)

func (r *Provider) Init(ctx context.Context, execpath, providerName string) error {
	log := log.FromContext(ctx)
	log.Debug("init provider", "execpath", execpath)
	r.Name = providerName
	r.Initializer = ProviderInitializer(execpath)
	r.Resources = sets.New[string]()
	r.ReadDataSources = sets.New[string]()
	r.ListDataSources = sets.New[string]()
	// initialize the provider
	provider, err := r.Initializer()
	if err != nil {
		log.Error("failed starting provider", "name", providerName, "error", err.Error())
		return fmt.Errorf("failed starting provider %s, err: %s", providerName, err.Error())
	}
	defer provider.Close(ctx)
	capResp, err := provider.Capabilities(ctx, &kfplugin1.Capabilities_Request{})
	if err != nil {
		log.Error("cannot get provider capabilities", "name", providerName)
		return fmt.Errorf("cannot get provider %s, capabilities, err: %s", providerName, err.Error())
	}

	if len(capResp.Resources) > 0 {
		log.Debug("resources", "name", providerName, "resources", capResp.Resources)
		r.Resources.Insert(capResp.Resources...)
	}
	if len(capResp.ReadDataSources) > 0 {
		log.Debug("read data sources", "name", providerName, "resources", capResp.ReadDataSources)
		r.ReadDataSources.Insert(capResp.ReadDataSources...)
	}
	if len(capResp.ListDataSources) > 0 {
		log.Debug("list data sources", "name", providerName, "resources", capResp.ListDataSources)
		r.ListDataSources.Insert(capResp.ListDataSources...)
	}
	return nil
}

// ProviderInitializer produces a provider factory that runs up the executable
// file in the given path and uses go-plugin to implement
// Provider Interface against it.
func ProviderInitializer(execPath string) Initializer {
	return func() (kfplugin.Provider, error) {

		client := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig:  kfplugin.Handshake,
			VersionedPlugins: kfplugin.VersionedPlugins,
			//AutoMTLS:         enableProviderAutoMTLS,
			Cmd: exec.Command(execPath),
			//Cmd:        exec.Command("./bin/provider-kubernetes"),
			SyncStdout: logging.PluginOutputMonitor(fmt.Sprintf("%s:stdout", "test")),
			SyncStderr: logging.PluginOutputMonitor(fmt.Sprintf("%s:stderr", "test")),
		})

		// Connect via RPC
		rpcClient, err := client.Client()
		if err != nil {
			return nil, err
		}

		// Request the plugin
		raw, err := rpcClient.Dispense(kfplugin.ProviderPluginName)
		if err != nil {
			return nil, err
		}

		p := raw.(*kfplugin.GRPCProvider)
		p.PluginClient = client
		return p, nil
	}
}
