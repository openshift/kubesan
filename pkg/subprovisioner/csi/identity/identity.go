// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/config"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type IdentityServer struct {
	csi.UnimplementedIdentityServer
}

func (s *IdentityServer) GetPluginInfo(ctx context.Context, in *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	resp := &csi.GetPluginInfoResponse{
		Name:          config.Domain,
		VendorVersion: config.Version,
	}
	return resp, nil
}

func (s *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	caps := []*csi.PluginCapability{
		{
			Type: &csi.PluginCapability_Service_{
				Service: &csi.PluginCapability_Service{
					Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
				},
			},
		},
	}

	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: caps,
	}
	return resp, nil
}

func (s *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	resp := &csi.ProbeResponse{
		Ready: &wrapperspb.BoolValue{
			Value: true,
		},
	}
	return resp, nil
}
