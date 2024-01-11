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
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/common/volume"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	info := volume.InfoFromString(req.VolumeId)

	// determine which node is the primary

	primary, err := s.getPrimary(ctx, info)
	if err != nil {
		return nil, err
	}

	// actually unstage volume

	if primary == s.NodeName {
		err = s.unstagePrimary(ctx, req, info)
	} else {
		err = s.unstageSecondary(ctx, req, info, primary)
	}

	if err != nil {
		return nil, err
	}

	// success

	resp := &csi.NodeUnstageVolumeResponse{}
	return resp, nil
}

func (s *NodeServer) getPrimary(ctx context.Context, info *volume.Info) (string, error) {
	pv, err := s.Clientset.CoreV1().PersistentVolumes().Get(ctx, info.PvName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	primary, _ := pv.Annotations[config.PvPrimaryAnnotation]

	return primary, nil
}

func (s *NodeServer) unstagePrimary(ctx context.Context, req *csi.NodeUnstageVolumeRequest, info *volume.Info) error {
	// refuse to unstage if other nodes are connected to the volume's NBD server (TODO: lift this restriction)

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: info.PvName}}

	err := k8s.AtomicUpdate(
		ctx, s.Clientset.CoreV1().RESTClient(), "persistentvolumes", pv,
		func(pv *corev1.PersistentVolume) error {
			if clients, _ := pv.Annotations[config.PvSecondariesAnnotation]; clients != "" {
				return status.Error(codes.Unavailable, "cannot unstage from primary node while staged on secondary nodes")
			}

			pv.Annotations[config.PvPrimaryAnnotation] = ""

			return nil
		},
	)
	if err != nil {
		return err
	}

	// deactivate LVM thin LV and LVM thin pool LV

	output, err := lvm.Command(
		ctx,
		"lvchange",
		"--devices", info.LvmPvPath,
		"--activate", "n",
		info.LvmThinLvRef(),
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM thin LV: %s: %s", err, output)
	}

	output, err = lvm.Command(
		ctx,
		"lvchange",
		"--devices", info.LvmPvPath,
		"--activate", "n",
		info.LvmThinPoolLvRef(),
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM thin pool LV: %s: %s", err, output)
	}

	// success

	return nil
}

func (s *NodeServer) unstageSecondary(ctx context.Context, req *csi.NodeUnstageVolumeRequest, info *volume.Info, primary string) error {
	// TODO: This is racy: if NodeUnstageVolume() fails after disconnecting the NBD device and is retried after the
	// same NBD device is used to stage a different volume, we will mess things up.

	// disconnect NBD device

	err := nbd.DisconnectClient(ctx, req.StagingTargetPath)
	if err != nil {
		return err
	}

	// remove this node as a secondary

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: info.PvName}}

	err = k8s.AtomicUpdate(
		ctx, s.Clientset.CoreV1().RESTClient(), "persistentvolumes", pv,
		func(pv *corev1.PersistentVolume) error {
			pv.Annotations[config.PvSecondariesAnnotation] =
				stringset.Remove(pv.Annotations[config.PvSecondariesAnnotation], s.NodeName)

			return nil
		},
	)
	if err != nil {
		return err
	}

	// terminate NBD server on the primary if there are no more secondaries

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

	if pv.Annotations[config.PvSecondariesAnnotation] == "" {
		err := nbdServerConfig.StopServer(ctx, s.Clientset)
		if err != nil {
			return err
		}
	}

	// success

	return nil
}
