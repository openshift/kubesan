// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"fmt"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/nbd"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/slices"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
)

// This method is idempotent and may be called from any node.
func (bm *BlobManager) DetachBlob(ctx context.Context, blob *Blob, node string, cookie string) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// update K8s PV annotation listing attachments

	nodeWithDirectAttachment, shouldCleanUpDirectAttachmentToCleanUp, indirectAttachmentsToCleanUp, err :=
		bm.unregisterAttachmentOnK8sPvAnnotation(ctx, blob, node, cookie)
	if err != nil {
		return err
	}

	// detach the volume

	for _, node := range indirectAttachmentsToCleanUp {
		err := bm.detachIndirect(ctx, blob, node, nodeWithDirectAttachment)
		if err != nil {
			return err
		}
	}

	if shouldCleanUpDirectAttachmentToCleanUp {
		err := bm.detachDirect(ctx, blob, nodeWithDirectAttachment)
		if err != nil {
			return err
		}
	}

	// success

	return nil
}

// This method is idempotent and may be called from any node.
func (bm *BlobManager) DetachBlobUnmanaged(ctx context.Context, blob *Blob, node string) error {
	return bm.detachDirect(ctx, blob, node)
}

// If node is nil, chooses any node, kinda.
func (bm *BlobManager) unregisterAttachmentOnK8sPvAnnotation(
	ctx context.Context, blob *Blob, node string, cookie string,
) (
	nodeWithDirectAttachment string,
	shouldCleanUpDirectAttachmentToCleanUp bool,
	indirectAttachmentsToCleanUp []string,
	err error,
) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	err = bm.atomicUpdateK8sPvForBlob(ctx, blob, func(pv *corev1.PersistentVolume) error {
		// get attachments annotation

		a, err := attachmentsFromK8sMeta(pv)
		if err != nil {
			return err
		}

		// add attachment

		nodeWithDirectAttachment = a.Direct.Node

		if a.Direct.Node == node {
			a.Direct.Holders = slices.Remove(a.Direct.Holders, cookie)
		} else {
			i, indirectAtt := a.findIndirectAttachmentOn(node)

			indirectAtt.Holders = slices.Remove(indirectAtt.Holders, cookie)
			if len(indirectAtt.Holders) == 0 {
				indirectAttachmentsToCleanUp = append(indirectAttachmentsToCleanUp, indirectAtt.Node)
				a.Indirect = slices.RemoveAt(a.Indirect, i)
			}
		}

		if len(a.Direct.Holders) == 0 && len(a.Indirect) == 0 {
			shouldCleanUpDirectAttachmentToCleanUp = true
			a.Direct = nil
		}

		// update attachments annotation

		err = a.setOnK8sMeta(pv)
		if err != nil {
			return err
		}

		return nil
	})

	return
}

func (bm *BlobManager) detachDirect(ctx context.Context, blob *Blob, node string) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// stop NBD server (which may have been started when creating an indirect attachment)

	nbdServerId := &nbd.ServerId{
		NodeName: node,
		BlobName: blob.Name,
	}

	err := nbd.StopServer(ctx, bm.clientset, nbdServerId)
	if err != nil {
		return err
	}

	// deactivate LVM thin and thin pool LVs

	job := &jobs.Job{
		Name:     fmt.Sprintf("deactivate-lv-%s", util.Hash(node, blob.lvmThinLvName())),
		NodeName: node,
		Command: []string{
			"scripts/lvm.sh", "deactivate",
			blob.BackingDevicePath, blob.lvmThinPoolLvName(), blob.lvmThinLvName(),
		},
	}

	err = jobs.CreateAndRunAndDelete(ctx, bm.clientset, job)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM LVs: %s", err)
	}

	// success

	return nil
}

func (bm *BlobManager) detachIndirect(
	ctx context.Context, blob *Blob, node string, nodeWithDirectAttachment string,
) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// tear down NBD client

	nbdServerId := &nbd.ServerId{
		NodeName: nodeWithDirectAttachment,
		BlobName: blob.Name,
	}

	err := nbd.DisconnectClient(ctx, bm.clientset, node, nbdServerId)
	if err != nil {
		return err
	}

	// success

	return nil
}
