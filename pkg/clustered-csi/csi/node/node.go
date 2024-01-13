// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/clustered-csi/clustered-csi/pkg/clustered-csi/csi/common/lvm"
	"gitlab.com/clustered-csi/clustered-csi/pkg/clustered-csi/csi/common/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type NodeServer struct {
	csi.UnimplementedNodeServer
	Clientset *util.Clientset
	NodeName  string
	Image     string
}

func (s *NodeServer) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	resp := &csi.NodeGetInfoResponse{
		NodeId: s.NodeName,
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

	vgName := strings.Split(req.VolumeId, "/")[0]
	thinPoolLvRef := fmt.Sprintf("%s-thin-pool", req.VolumeId)
	thinLvRef := fmt.Sprintf("%s-thin", req.VolumeId)

	// ensure the volume group's lockspace is started

	err := lvm.StartVgLockspace(ctx, vgName)
	if err != nil {
		return nil, err
	}

	// activate thin pool and thin volume

	output, err := lvm.Command(ctx, "lvchange", "--activate", "ey", thinPoolLvRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to activate thin pool: %s: %s", err, output)
	}

	output, err = lvm.Command(ctx, "lvchange", "--activate", "ey", thinLvRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to activate thin volume: %s: %s", err, output)
	}

	// create symlink to thin volume where Kubernetes expects it

	output, err = lvm.Command(ctx, "lvs", "--options", "lv_path", "--noheadings", thinLvRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get path to thin volume: %s: %s", err, output)
	}

	lv_path := strings.TrimSpace(output)

	err = util.Symlink(lv_path, req.StagingTargetPath)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodeStageVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// deactivate thin volume and thin pool

	thinPoolLvRef := fmt.Sprintf("%s-thin-pool", req.VolumeId)
	thinLvRef := fmt.Sprintf("%s-thin", req.VolumeId)

	output, err := lvm.Command(ctx, "lvchange", "--activate", "n", thinLvRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deactivate thin volume: %s: %s", err, output)
	}

	output, err = lvm.Command(ctx, "lvchange", "--activate", "n", thinPoolLvRef)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deactivate thin pool: %s: %s", err, output)
	}

	// success

	resp := &csi.NodeUnstageVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// TODO: Validate request.

	// create symlink to thin volume where Kubernetes expects it

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
