// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"errors"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.com/kubesan/kubesan/internal/csi/common/validate"
)

func (s *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// validate request

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify volume id")
	}

	if req.TargetPath == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify target path")
	}

	if req.VolumeCapability == nil {
		return nil, status.Errorf(codes.InvalidArgument, "must specify volume capability")
	}

	if err := validate.ValidateVolumeCapability(req.VolumeCapability); err != nil {
		return nil, err
	}

	// create symlink to device for Kubernetes (replace if there is a file, empty dir, or symlink in its place;
	// Kubernetes places an empty directory at the path where block volumes should be staged/published)

	err := os.Remove(req.TargetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	err = os.Symlink(req.StagingTargetPath, req.TargetPath)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodePublishVolumeResponse{}

	return resp, nil
}

func (s *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// validate request

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify volume id")
	}

	if req.TargetPath == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify target path")
	}

	// remove symlink created by NodePublishVolume

	err := os.Remove(req.TargetPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// success

	resp := &csi.NodeUnpublishVolumeResponse{}

	return resp, nil
}
