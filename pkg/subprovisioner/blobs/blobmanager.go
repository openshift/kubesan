// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"strconv"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/lvm"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type BlobManager struct {
	clientset *k8s.Clientset
}

func NewBlobManager(clientset *k8s.Clientset) *BlobManager {
	return &BlobManager{clientset: clientset}
}

// This method may be called from any node, and fails if the blob does not exist.
func (bm *BlobManager) GetBlobSize(ctx context.Context, blob *Blob) (int64, error) {
	output, err := lvm.Command(
		ctx,
		"lvs",
		"--devices", blob.BackingDevicePath,
		"--options", "lv_size",
		"--units", "b",
		"--nosuffix",
		"--noheadings",
		blob.lvmThinLvRef(),
	)
	if err != nil {
		return -1, status.Errorf(codes.Internal, "failed to get size of LVM LV: %s: %s", err, output)
	}

	sizeStr := strings.TrimSpace(output)

	size, err := strconv.ParseInt(sizeStr, 0, 64)
	if err != nil {
		return -1, status.Errorf(codes.Internal, "failed to get size of LVM LV: %s: %s", err, sizeStr)
	}

	return size, nil
}

// The returned path is valid on all nodes in the cluster.
//
// This method may be called from any node, and fails if the volume does not exist.
func (bm *BlobManager) getBlobLvmThinLvPath(ctx context.Context, blob *Blob) (string, error) {
	output, err := lvm.Command(
		ctx,
		"lvs",
		"--devices", blob.BackingDevicePath,
		"--options", "lv_path",
		"--noheadings",
		blob.lvmThinLvRef(),
	)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to get path to LVM LV: %s: %s", err, output)
	}

	return strings.TrimSpace(output), nil
}
