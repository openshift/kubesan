// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/lvm"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/volume"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type ControllerServer struct {
	csi.UnimplementedControllerServer
	Clientset *k8s.Clientset
	Image     string
}

func (s *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	caps := []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_SINGLE_NODE_MULTI_WRITER,
	}

	csiCaps := make([]*csi.ControllerServiceCapability, len(caps))
	for i, cap := range caps {
		csiCaps[i] = &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: csiCaps,
	}
	return resp, nil
}

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

	info := volume.Info{
		LvmPvPath: backingDevicePath,
		PvName:    pvName,
		PvcUid:    pvc.UID,
	}

	// ensure the volume group has been created

	err = s.ensureVgIsCreated(ctx, *pvc.Spec.StorageClassName, info.LvmPvPath)
	if err != nil {
		return nil, err
	}

	// ensure the volume group's lockspace is started

	err = lvm.StartVgLockspace(ctx, info.LvmPvPath)
	if err != nil {
		return nil, err
	}

	// create LVM thin pool LV and LVM thin LV

	size := fmt.Sprintf("%db", capacity)

	output, err := lvm.IdempotentLvCreate(
		ctx,
		"--devices", info.LvmPvPath,
		"--activate", "n",
		"--type", "thin-pool",
		"--name", info.LvmThinPoolLvName(),
		"--size", size,
		config.LvmVgName,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create LVM thin pool LV: %s: %s", err, output)
	}

	output, err = lvm.IdempotentLvCreate(
		ctx,
		"--devices", info.LvmPvPath,
		"--type", "thin",
		"--name", info.LvmThinLvName(),
		"--thinpool", info.LvmThinPoolLvName(),
		"--virtualsize", size,
		config.LvmVgName,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create LVM thin LV: %s: %s", err, output)
	}

	// deactivate LVM thin LV (`--activate n` has no effect on `lvcreate --type thin`)

	output, err = lvm.Command(
		ctx,
		"lvchange",
		"--devices", info.LvmPvPath,
		"--activate", "n",
		info.LvmThinLvRef(),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deactivate LVM thin LV: %s: %s", err, output)
	}

	// success

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: capacity,
			VolumeId:      info.ToString(),
			VolumeContext: map[string]string{},
			ContentSource: req.VolumeContentSource,
		},
	}
	return resp, nil
}

func (s *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// validate request

	if req.VolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify volume id")
	}

	info := volume.InfoFromString(req.VolumeId)

	// ensure the volume group's lockspace is started

	err := lvm.StartVgLockspace(ctx, info.LvmPvPath)
	if err != nil {
		return nil, err
	}

	// remove LVM thin LV and LVM thin pool LV

	output, err := lvm.IdempotentLvRemove(ctx, "--devices", info.LvmPvPath, info.LvmThinLvRef())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove LVM thin LV: %s: %s", err, output)
	}

	output, err = lvm.IdempotentLvRemove(ctx, "--devices", info.LvmPvPath, info.LvmThinPoolLvRef())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove LVM thin pool LV: %s: %s", err, output)
	}

	// success

	resp := &csi.DeleteVolumeResponse{}
	return resp, nil
}

func (s *ControllerServer) ensureVgIsCreated(ctx context.Context, storageClassName string, pvPath string) error {
	// TODO: This will hang if the CSI controller plugin creating the VG dies. Fix this, maybe using leases.
	// TODO: Communicate VG creation errors to users through events/status on the SC and PVC.

	storageClasses := s.Clientset.StorageV1().StorageClasses()
	stateAnnotation := fmt.Sprintf("%s/vg-state", config.Domain)

	// check VG state

	var sc *storagev1.StorageClass
	var shouldCreateVg bool

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var err error
		sc, err = storageClasses.Get(ctx, storageClassName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		switch state, _ := sc.Annotations[stateAnnotation]; state {
		case "", "creation-failed":
			// VG wasn't created and isn't being created, try to create it ourselves

			if sc.Annotations == nil {
				sc.Annotations = map[string]string{}
			}
			sc.Annotations[stateAnnotation] = "creating"

			sc, err = storageClasses.Update(ctx, sc, metav1.UpdateOptions{})
			if err != nil {
				return err
			}

			shouldCreateVg = true

		case "creating", "created":
			shouldCreateVg = false
		}

		return nil
	})
	if err != nil {
		return err
	}

	// create VG or wait until it is created

	if shouldCreateVg {
		_, err = lvm.Command(ctx, "vgcreate", "--lock-type", "sanlock", config.LvmVgName, pvPath)

		if err == nil {
			sc.Annotations[stateAnnotation] = "created"
		} else {
			sc.Annotations[stateAnnotation] = "creation-failed"
		}

		// don't use ctx so that we don't fail to update the annotation after successfully creating the VG
		// TODO: This fails if the SC was modified meanwhile, fix this.
		_, err = storageClasses.Update(context.Background(), sc, metav1.UpdateOptions{})
		return err
	} else {
		// TODO: Watch instead of polling.
		for {
			sc, err := storageClasses.Get(ctx, storageClassName, metav1.GetOptions{})

			if err != nil {
				return err
			} else if ctx.Err() != nil {
				return ctx.Err()
			} else if sc.Annotations[stateAnnotation] == "created" {
				return nil
			}

			time.Sleep(1 * time.Second)
		}
	}
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
