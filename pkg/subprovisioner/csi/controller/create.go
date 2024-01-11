// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/volumemanager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// TODO: Reject unknown parameters in req.Parameters that *don't* start with `csi.storage.k8s.io/`.

	// validate request

	if req.VolumeContentSource != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported volume content source")
	}

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

	// retrieve PVC so we can get its UID

	pvc, err := s.Clientset.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	volume := &volumemanager.Volume{
		K8sPvcUid:         pvc.UID,
		K8sPvName:         pvName,
		BackingDevicePath: backingDevicePath,
	}

	err = s.VolumeManager.CreateVolume(ctx, volume, *pvc.Spec.StorageClassName, capacity)
	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: capacity,
			VolumeId:      volume.ToString(),
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
