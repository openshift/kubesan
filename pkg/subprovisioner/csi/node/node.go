// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/fs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/volumemanager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeServer struct {
	csi.UnimplementedNodeServer
	Clientset     *k8s.Clientset
	VolumeManager *volumemanager.VolumeManager
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

func (s *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// TODO: Validate request.
	// TODO: Must enforce access modes ourselves; check the CSI spec.

	// validate request

	if req.VolumeCapability.GetBlock() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected a block volume")
	}

	volume, err := volumemanager.VolumeFromString(req.VolumeId)
	if err != nil {
		return nil, err
	}

	// attach volume to current node

	_, path, err := s.VolumeManager.AttachVolume(ctx, volume, &config.LocalNodeName, "staged")
	if err != nil {
		return nil, err
	}

	// create symlink to device for NodePublishVolume()

	err = fs.Symlink(path, req.StagingTargetPath)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodeStageVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// validate request

	volume, err := volumemanager.VolumeFromString(req.VolumeId)
	if err != nil {
		return nil, err
	}

	// detach volume from current node

	err = s.VolumeManager.DetachVolume(ctx, volume, config.LocalNodeName, "staged")
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodeUnstageVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// TODO: Validate request.

	// create symlink to device for Kubernetes

	err := fs.Symlink(req.StagingTargetPath, req.TargetPath)
	if err != nil {
		return nil, err
	}

	resp := &csi.NodePublishVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// TODO: Validate request.

	resp := &csi.NodeUnpublishVolumeResponse{}
	return resp, nil
}
