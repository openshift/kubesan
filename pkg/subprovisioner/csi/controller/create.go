// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/volumemanager"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// TODO: Reject unknown parameters in req.Parameters that *don't* start with `csi.storage.k8s.io/`.

	// validate request

	switch {
	case req.VolumeContentSource == nil:
		// ok, no volume content source
	case req.VolumeContentSource.GetVolume() != nil:
		// ok, volume content source is another volume
	default:
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

	// populate LVM thin LV

	if source := req.VolumeContentSource.GetVolume(); source != nil {
		sourceVolume, err := volumemanager.VolumeFromString(source.VolumeId)
		if err != nil {
			return nil, err
		}

		err = s.cloneVolume(ctx, sourceVolume, volume)
		if err != nil {
			return nil, err
		}
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

func (s *ControllerServer) cloneVolume(ctx context.Context, sourceVol *volumemanager.Volume, targetVol *volumemanager.Volume) error {
	// TODO: Ensure that target isn't smaller than source.

	// attach both volumes (preferring a node where there already is a primary attachment for the source volume)

	cookie := fmt.Sprintf("cloning-into-%s", targetVol.K8sPvcUid)

	nodeName, sourcePathOnHost, err := s.VolumeManager.AttachVolume(ctx, sourceVol, nil, cookie)
	if err != nil {
		return err
	}

	_, targetPathOnHost, err := s.VolumeManager.AttachVolumeUnmanaged(ctx, targetVol, &nodeName)
	if err != nil {
		return err
	}

	// run clone job

	job := &jobs.Job{
		Name:     fmt.Sprintf("populate-volume-%s", targetVol.K8sPvcUid),
		NodeName: nodeName,
		Command: []string{
			"dd",
			fmt.Sprintf("if=%s", sourcePathOnHost),
			fmt.Sprintf("of=%s", targetPathOnHost),
			"bs=1M",
			"conv=fsync,nocreat,sparse",
		},
	}

	err = jobs.Run(ctx, s.Clientset, job)
	if err != nil {
		return err
	}

	// detach both volumes

	err = s.VolumeManager.DetachVolumeUnmanaged(ctx, targetVol, nodeName)
	if err != nil {
		return err
	}

	err = s.VolumeManager.DetachVolume(ctx, sourceVol, nodeName, cookie)
	if err != nil {
		return err
	}

	// success

	return nil
}
