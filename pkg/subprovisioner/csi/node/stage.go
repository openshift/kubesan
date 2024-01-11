// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/lvm"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/nbd"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/stringset"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/util"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/volume"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// TODO: Validate request.
	// TODO: Must enforce access modes ourselves; check the CSI spec.

	// validate request

	if req.VolumeCapability.GetBlock() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "expected a block volume")
	}

	info := volume.InfoFromString(req.VolumeId)

	// ensure the volume group's lockspace is started

	err := lvm.StartVgLockspace(ctx, info.LvmPvPath)
	if err != nil {
		return nil, err
	}

	// activate volume on current node if needed

	primary, err := s.getOrBecomePrimary(ctx, req, info)
	if err != nil {
		return nil, err
	}

	// actually stage volume

	if primary == s.NodeName {
		err = s.stagePrimary(ctx, req, info)
	} else {
		err = s.stageSecondary(ctx, req, info, primary)
	}

	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodeStageVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) getOrBecomePrimary(ctx context.Context, req *csi.NodeStageVolumeRequest, info *volume.Info) (string, error) {
	// TODO: This doesn't properly handle cases where the node plugin dies or staging is cancelled and so on.

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: info.PvName}}

	err := k8s.AtomicUpdate(
		ctx, s.Clientset.CoreV1().RESTClient(), "persistentvolumes", pv,
		func(pv *corev1.PersistentVolume) error {
			if pv.Annotations == nil {
				pv.Annotations = map[string]string{}
			}

			if primary, _ := pv.Annotations[config.PvPrimaryAnnotation]; primary == "" {
				pv.Annotations[config.PvPrimaryAnnotation] = s.NodeName
			}

			return nil
		},
	)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to set PV annotation: %s", err)
	}

	return pv.Annotations[config.PvPrimaryAnnotation], nil
}

func (s *NodeServer) stagePrimary(ctx context.Context, req *csi.NodeStageVolumeRequest, info *volume.Info) error {
	// activate LVM thin pool LV and LVM thin LV

	output, err := lvm.Command(
		ctx,
		"lvchange",
		"--devices", info.LvmPvPath,
		"--activate", "ey",
		info.LvmThinPoolLvRef(),
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to activate LVM thin pool LV: %s: %s", err, output)
	}

	output, err = lvm.Command(
		ctx,
		"lvchange",
		"--devices", info.LvmPvPath,
		"--activate", "ey",
		info.LvmThinLvRef(),
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to activate LVM thin LV: %s: %s", err, output)
	}

	// create symlink to LVM thin LV where Kubernetes expects it to be

	lvmLvPath, err := lvm.GetLvPath(ctx, info)
	if err != nil {
		return err
	}

	err = util.Symlink(lvmLvPath, req.StagingTargetPath)
	if err != nil {
		return err
	}

	// success

	return nil
}

func (s *NodeServer) stageSecondary(ctx context.Context, req *csi.NodeStageVolumeRequest, info *volume.Info, primary string) error {
	// add ourselves to the client list on the PV

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: info.PvName}}

	err := k8s.AtomicUpdate(
		ctx, s.Clientset.CoreV1().RESTClient(), "persistentvolumes", pv,
		func(pv *corev1.PersistentVolume) error {
			if pv.Annotations == nil {
				pv.Annotations = map[string]string{}
			}

			pv.Annotations[config.PvSecondariesAnnotation] =
				stringset.Insert(pv.Annotations[config.PvSecondariesAnnotation], s.NodeName)

			return nil
		},
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to set PV annotation: %s", err)
	}

	// start NBD server on primary node

	lvmLvPath, err := lvm.GetLvPath(ctx, info)
	if err != nil {
		return err
	}

	nbdServerConfig := nbd.ServerConfig{
		PvcUid:           info.PvcUid,
		NodeName:         primary,
		DevicePathOnHost: lvmLvPath,
		Image:            s.Image,
	}

	err = nbdServerConfig.StartServer(ctx, s.Clientset)
	if err != nil {
		return err
	}

	// set up NBD client on current node

	nbdDevicePath, err := nbd.ConnectClient(ctx, nbdServerConfig.Hostname())
	if err != nil {
		return err
	}

	// create symlink to NBD device where Kubernetes expects it to be

	err = util.Symlink(nbdDevicePath, req.StagingTargetPath)
	if err != nil {
		return err
	}

	// success

	return nil
}
