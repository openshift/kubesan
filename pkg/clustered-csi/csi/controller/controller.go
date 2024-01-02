// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/clustered-csi/clustered-csi/pkg/clustered-csi/csi/common/lvm"
	"gitlab.com/clustered-csi/clustered-csi/pkg/clustered-csi/csi/common/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ControllerServer struct {
	csi.UnimplementedControllerServer
	Clientset *util.Clientset
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

		switch cap.AccessMode.Mode {
		case csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
			csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY,
			csi.VolumeCapability_AccessMode_SINGLE_NODE_SINGLE_WRITER,
			csi.VolumeCapability_AccessMode_SINGLE_NODE_MULTI_WRITER:
		default:
			return nil, status.Errorf(
				codes.InvalidArgument,
				"only access modes ReadWriteOnce and ReadWriteOncePod are supported",
			)
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

	pvcName, err := getParameter("csi.storage.k8s.io/pvc/name")
	if err != nil {
		return nil, err
	}
	pvcNamespace, err := getParameter("csi.storage.k8s.io/pvc/namespace")
	if err != nil {
		return nil, err
	}
	lvmVolumeGroupName, err := getParameter("lvmVolumeGroupName")
	if err != nil {
		return nil, err
	}

	// retrieve PVC so we can get its UID

	pvc, err := s.Clientset.CoreV1().PersistentVolumeClaims(pvcNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	volumeId := fmt.Sprintf("%s/%s", lvmVolumeGroupName, pvc.UID)

	// create thin pool and thin volume

	thin_pool_name := fmt.Sprintf("%s-thin-pool", pvc.UID)
	thin_lv_name := fmt.Sprintf("%s-thin", pvc.UID)
	size := fmt.Sprintf("%db", capacity)

	output, err := lvm.Command(
		"lvcreate",
		"--activate", "n",
		"--type", "thin-pool",
		"--name", thin_pool_name,
		"--size", size,
		lvmVolumeGroupName,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create thin pool: %s: %s", err, output)
	}

	output, err = lvm.Command(
		"lvcreate",
		"--type", "thin",
		"--name", thin_lv_name,
		"--thinpool", thin_pool_name,
		"--virtualsize", size,
		lvmVolumeGroupName,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create thin volume: %s: %s", err, output)
	}

	// deactivate thin volume (`--activate n` has no effect on `lvcreate --type thin`)

	thin_lv_ref := fmt.Sprintf("%s-thin", volumeId)

	output, err = lvm.Command("lvchange", "--activate", "n", thin_lv_ref)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to deactivate thin volume: %s: %s", err, output)
	}

	// success

	resp := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			CapacityBytes: capacity,
			VolumeId:      volumeId,
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

	// remove thin volume and thin pool

	thin_pool_ref := fmt.Sprintf("%s-thin-pool", req.VolumeId)
	thin_lv_ref := fmt.Sprintf("%s-thin", req.VolumeId)

	output, err := lvm.Command("lvremove", thin_lv_ref)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove thin volume: %s: %s", err, output)
	}

	output, err = lvm.Command("lvremove", thin_pool_ref)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to remove thin pool: %s: %s", err, output)
	}

	// success

	resp := &csi.DeleteVolumeResponse{}
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
