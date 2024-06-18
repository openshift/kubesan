// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/util"
)

func (s *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// TODO: Validate request.

	// create symlink to device for Kubernetes

	err := util.Symlink(req.StagingTargetPath, req.TargetPath)
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
