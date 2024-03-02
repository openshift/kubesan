// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/blobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// TODO: Reject unknown parameters in req.Parameters that *don't* start with `csi.storage.k8s.io/`.

	// validate request

	for _, cap := range req.VolumeCapabilities {
		if cap.GetBlock() == nil {
			return nil, status.Errorf(codes.InvalidArgument, "only block volumes are supported")
		}
	}

	capacity, _, _, err := validateCapacity(req.CapacityRange)
	if err != nil {
		return nil, err
	}

	getParameter := func(key string) (string, error) {
		value := req.Parameters[key]
		if value == "" {
			return "", status.Errorf(codes.InvalidArgument, "missing/empty parameter \"%s\"", key)
		}
		return value, nil
	}

	pvName, err := getParameter("csi.storage.k8s.io/pv/name")
	if err != nil {
		return nil, err
	}
	pvcName, err := getParameter("csi.storage.k8s.io/pvc/name")
	if err != nil {
		return nil, err
	}
	pvcNamespace, err := getParameter("csi.storage.k8s.io/pvc/namespace")
	if err != nil {
		return nil, err
	}
	backingDevicePath, err := getParameter("backingDevicePath")
	if err != nil {
		return nil, err
	}

	// retrieve PVC so we can get its StorageClass

	pvc, err := s.Clientset.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal, "failed to get PVC \"%s\" in namespace \"%s\": %s", pvcName, pvcNamespace, err,
		)
	}

	// create blob

	blob := &blobs.Blob{
		Name:                    pvName,
		K8sPersistentVolumeName: pvName,
		BackingDevicePath:       backingDevicePath,
	}

	err = s.BlobManager.CreateBlobEmpty(ctx, blob, *pvc.Spec.StorageClassName, capacity)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create empty blob \"%s\": %s", blob, err)
	}

	// populate blob

	if req.VolumeContentSource != nil {
		var sourceBlob *blobs.Blob

		if source := req.VolumeContentSource.GetVolume(); source != nil {
			sourceBlob, err = blobs.BlobFromString(source.VolumeId)
		} else if source := req.VolumeContentSource.GetSnapshot(); source != nil {
			sourceBlob, err = blobs.BlobFromString(source.SnapshotId)
		} else {
			return nil, status.Errorf(codes.InvalidArgument, "unsupported volume content source")
		}

		if err != nil {
			return nil, err
		}

		err = s.populateVolume(ctx, sourceBlob, blob)
		if err != nil {
			return nil, err
		}
	}

	// success

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: capacity,
			VolumeId:      blob.String(),
			VolumeContext: map[string]string{},
			ContentSource: req.VolumeContentSource,
		},
	}
	return resp, nil
}

func validateCapacity(capacityRange *csi.CapacityRange) (capacity int64, minCapacity int64, maxCapacity int64, err error) {
	if capacityRange == nil {
		return -1, -1, -1, status.Errorf(codes.InvalidArgument, "must specify capacity")
	}

	minCapacity = capacityRange.RequiredBytes
	maxCapacity = capacityRange.LimitBytes

	if minCapacity == 0 {
		return -1, -1, -1, status.Errorf(codes.InvalidArgument, "must specify minimum capacity")
	}
	if maxCapacity != 0 && maxCapacity < minCapacity {
		return -1, -1, -1, status.Errorf(codes.InvalidArgument, "minimum capacity must not exceed maximum capacity")
	}

	// TODO: Check for overflow.
	capacity = (minCapacity + 511) / 512 * 512

	if maxCapacity != 0 && maxCapacity < capacity {
		return -1, -1, -1, status.Errorf(codes.InvalidArgument, "actual capacity must be a multiple of 512 bytes")
	}

	return
}

func (s *ControllerServer) populateVolume(ctx context.Context, sourceBlob *blobs.Blob, targetBlob *blobs.Blob) error {
	// TODO: Ensure that target isn't smaller than source.

	// attach both blobs (preferring a node where there already is a direct attachment for the source blob)

	cookie := fmt.Sprintf("copying-to-%s", targetBlob.Name)

	nodeName, sourcePathOnHost, err := s.BlobManager.AttachBlob(ctx, sourceBlob, nil, cookie)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to attach blob \"%s\": %s", sourceBlob, err)
	}

	_, targetPathOnHost, err := s.BlobManager.AttachBlobUnmanaged(ctx, targetBlob, &nodeName)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to attach blob \"%s\": %s", targetBlob, err)
	}

	// run population job

	job := &jobs.Job{
		Name:     fmt.Sprintf("populate-%s", targetBlob.Name),
		NodeName: nodeName,
		Command: []string{
			"dd",
			fmt.Sprintf("if=%s", sourcePathOnHost),
			fmt.Sprintf("of=%s", targetPathOnHost),
			"bs=1M",
			"conv=fsync,nocreat,sparse",
		},
	}

	err = jobs.CreateAndRun(ctx, s.Clientset, job)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to populate blob \"%s\": %s", targetBlob, err)
	}

	// detach both blobs

	err = s.BlobManager.DetachBlobUnmanaged(ctx, targetBlob, nodeName)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to detach blob \"%s\": %s", targetBlob, err)
	}

	err = s.BlobManager.DetachBlob(ctx, sourceBlob, nodeName, cookie)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to detach blob \"%s\": %s", sourceBlob, err)
	}

	// success

	return nil
}

func (s *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// validate request

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify volume id")
	}

	blob, err := blobs.BlobFromString(req.VolumeId)
	if err != nil {
		return nil, err
	}

	// delete population job (if any)

	err = jobs.Delete(ctx, s.Clientset, fmt.Sprintf("populate-%s", blob.Name))
	if err != nil {
		return nil, err
	}

	// delete blob

	err = s.BlobManager.DeleteBlob(ctx, blob)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete blob \"%s\": %s", blob, err)
	}

	// success

	resp := &csi.DeleteVolumeResponse{}
	return resp, nil
}
