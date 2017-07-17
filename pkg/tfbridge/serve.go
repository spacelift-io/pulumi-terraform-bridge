// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package tfbridge

import (
	goplugin "github.com/hashicorp/go-plugin"
	"github.com/pulumi/lumi/pkg/resource/provider"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
)

// Serve dynamically loads a Terraform plugin, fires up a Lumi resource provider listening to inbound gRPC traffic,
// and actively bridges between the two, translating calls on one side into calls on the other.
func Serve(module string, info ProviderInfo) error {
	var plug *goplugin.Client
	defer (func() {
		// If the plugin was initialized, we want to kill it when the program exits.
		if plug != nil {
			plug.Kill()
		}
	})()

	// Create a new resource provider server and listen for and serve incoming connections.
	return provider.Main(func(host *provider.HostClient) (lumirpc.ResourceProviderServer, error) {
		// Manufacture the path to the provider binary.
		provBin := "terraform-provider-" + module

		// Load up the Terraform plugin dynamically so we can invoke CRUD methods on it.
		plug, err := Plugin(host, provBin)
		if err != nil {
			return nil, err
		}

		// Put the plugin into debug-only output mode and enable real output just prior to returning.  This is to avoid
		// spewing configuration information to the output until we're actually performing CRUD operations.
		plug.Logger.Disable()
		defer plug.Logger.Enable()

		// Create a new bridge provider.
		bridge := NewProvider(host, module, plug.Provider, info)

		// Configure the provider with all of the state on the Lumi side.
		// TODO: the semantics of this aren't quite what we want.  We already have some oddness around configuration
		//     initialization due to module initialization order.  But it's even weirder now because when we
		//     take this snapshot from the heap is semi-arbitrary.  We need to revist configuration state lifetime.
		if err = bridge.Configure(); err != nil {
			// TODO: errors need to come back in a friendlier way.  Perhaps Configure belongs on the RPC interface.
			return nil, err
		}

		return bridge, nil
	})
}
