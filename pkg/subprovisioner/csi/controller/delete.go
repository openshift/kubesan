// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/volumemanager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// validate request

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify volume id")
	}

	volume, err := volumemanager.VolumeFromString(req.VolumeId)
	if err != nil {
		return nil, err
	}

	// delete volume

	err = s.VolumeManager.DeleteVolume(ctx, volume)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.DeleteVolumeResponse{}
	return resp, nil
}
