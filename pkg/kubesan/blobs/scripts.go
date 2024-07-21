// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"
	"strconv"

	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/jobs"
	"gitlab.com/kubesan/kubesan/pkg/kubesan/util/util"
)

func (bm *BlobManager) runLvmScriptForThinPoolLv(
	ctx context.Context, blobPool *internalBlobPool, node string, command string, extraArgs ...string,
) error {
	job := &jobs.Job{
		Name:     fmt.Sprintf("%s-lv-%s", command, util.Hash(node, blobPool.lvmThinPoolLvName())),
		NodeName: node,
		Command: append(
			[]string{
				"scripts/lvm.sh", command,
				blobPool.backingVolumeGroup,
				blobPool.lvmThinPoolLvName(),
			},
			extraArgs...,
		),
		ServiceAccountName: "blobs",
		HostPID:            true,
	}

	err := jobs.CreateAndRunAndDelete(ctx, bm.clientset, job)
	if err != nil {
		return err
	}

	return nil
}

func (bm *BlobManager) runLvmScriptForThinLv(
	ctx context.Context, blob *Blob, node string, command string, extraArgs ...string,
) error {
	job := &jobs.Job{
		Name:     fmt.Sprintf("%s-lv-%s", command, util.Hash(node, blob.lvmThinLvName())),
		NodeName: node,
		Command: append(
			[]string{
				"scripts/lvm.sh", command,
				blob.pool.backingVolumeGroup,
				blob.pool.lvmThinPoolLvName(), blob.lvmThinLvName(),
			},
			extraArgs...,
		),
		ServiceAccountName: "blobs",
		HostPID:            true,
	}

	err := jobs.CreateAndRunAndDelete(ctx, bm.clientset, job)
	if err != nil {
		return err
	}

	return nil
}

func (bm *BlobManager) runDmMultipathScript(
	ctx context.Context, blob *Blob, node string, command string, extraArgs ...string,
) error {
	size, err := bm.GetBlobSize(ctx, blob)
	if err != nil {
		return err
	}

	job := &jobs.Job{
		Name:     fmt.Sprintf("%s-dm-mp-%s", command, util.Hash(node, blob.lvmThinLvName())),
		NodeName: node,
		Command: append(
			[]string{
				"scripts/dm-multipath.sh", command,
				blob.dmMultipathVolumeName(), strconv.FormatInt(size, 10)},
			extraArgs...,
		),
		ServiceAccountName: "blobs",
		HostPID:            true,
	}

	err = jobs.CreateAndRunAndDelete(ctx, bm.clientset, job)
	if err != nil {
		return err
	}

	return nil
}
