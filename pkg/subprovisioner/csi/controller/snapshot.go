// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/blobs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	// TODO: Reject unknown parameters in req.Parameters that *don't* start with `csi.storage.k8s.io/`.

	sourceVolumeBlob, err := blobs.BlobFromString(req.SourceVolumeId)
	if err != nil {
		return nil, err
	}

	// validate request

	getParameter := func(key string) (string, error) {
		value := req.Parameters[key]
		if value == "" {
			return "", status.Errorf(codes.InvalidArgument, "missing/empty parameter \"%s\"", key)
		}
		return value, nil
	}

	vscName, err := getParameter("csi.storage.k8s.io/volumesnapshotcontent/name")
	if err != nil {
		return nil, err
	}

	// create blob

	snapshotBlob, err := s.BlobManager.CreateBlobCopy(ctx, vscName, sourceVolumeBlob)
	if err != nil {
		return nil, err
	}

	size, err := s.BlobManager.GetBlobSize(ctx, snapshotBlob)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      size,
			SnapshotId:     snapshotBlob.String(),
			SourceVolumeId: req.SourceVolumeId,
			CreationTime:   timestamppb.Now(),
			ReadyToUse:     true,
		},
	}
	return resp, nil
}

func (s *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// validate request

	if req.SnapshotId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify snapshot id")
	}

	blob, err := blobs.BlobFromString(req.SnapshotId)
	if err != nil {
		return nil, err
	}

	// delete blob

	err = s.BlobManager.DeleteBlob(ctx, blob)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.DeleteSnapshotResponse{}
	return resp, nil
}
