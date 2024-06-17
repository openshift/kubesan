// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/blobs"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/config"
	"k8s.io/mount-utils"
	"k8s.io/utils/exec"
)

type NodeServer struct {
	csi.UnimplementedNodeServer
	BlobManager *blobs.BlobManager

	// Keep around instances to avoid the cost of probing in mount.New("")
	// each time an instance is required.
	mounter mount.Interface
	exec    exec.Interface
}

func NewNodeServer(blobManager *blobs.BlobManager) *NodeServer {
	return &NodeServer{
		BlobManager: blobManager,
		mounter:     mount.New(""),
		exec:        exec.New(),
	}
}

func (s *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	resp := &csi.NodeGetInfoResponse{
		NodeId: config.LocalNodeName,
	}
	return resp, nil
}

func (s *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	caps := []csi.NodeServiceCapability_RPC_Type{
		csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
		csi.NodeServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
	}

	csiCaps := make([]*csi.NodeServiceCapability, len(caps))
	for i, cap := range caps {
		csiCaps[i] = &csi.NodeServiceCapability{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	resp := &csi.NodeGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}
	return resp, nil
}
