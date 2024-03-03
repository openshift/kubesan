// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/blobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// TODO: Validate request.
	// TODO: Must enforce access modes ourselves; check the CSI spec.

	// validate request

	if req.VolumeCapability.GetBlock() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected a block volume")
	}

	blob, err := blobs.BlobFromString(req.VolumeId)
	if err != nil {
		return nil, err
	}

	// attach blob on current node

	_, path, err := s.BlobManager.AttachBlob(ctx, blob, &config.LocalNodeName, "staged")
	if err != nil {
		return nil, err
	}

	// create symlink to device for NodePublishVolume()

	err = util.Symlink(path, req.StagingTargetPath)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodeStageVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// validate request

	blob, err := blobs.BlobFromString(req.VolumeId)
	if err != nil {
		return nil, err
	}

	// detach blob from current node

	err = s.BlobManager.DetachBlob(ctx, blob, config.LocalNodeName, "staged")
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodeUnstageVolumeResponse{}
	return resp, nil
}
