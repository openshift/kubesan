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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This method is idempotent and may be called from any node.
func (bm *BlobManager) DetachBlob(ctx context.Context, blob *Blob, node string, cookie string) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	nodeWithDirectAttachment, shouldCleanUpDirectAttachment, indirectAttachmentsToCleanUp, err :=
		bm.whatShouldBeDetached(ctx, blob, node, cookie)
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

	if shouldCleanUpDirectAttachment {
		err := bm.detachDirect(ctx, blob, nodeWithDirectAttachment)
		if err != nil {
			return err
		}
	}

	// update K8s PV annotation listing attachments

	err = bm.unregisterAttachmentOnK8sPvAnnotation(ctx, blob, node, cookie)
	if err != nil {
		return err
	}

	// success

	return nil
}

// This method is idempotent and may be called from any node.
func (bm *BlobManager) DetachBlobUnmanaged(ctx context.Context, blob *Blob, node string) error {
	return bm.detachDirect(ctx, blob, node)
}

func (bm *BlobManager) whatShouldBeDetached(
	ctx context.Context, blob *Blob, node string, cookie string,
) (
	nodeWithDirectAttachment string,
	shouldCleanUpDirectAttachment bool,
	indirectAttachmentsToCleanUp []string,
	err error,
) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	pv, err := bm.clientset.CoreV1().PersistentVolumes().Get(ctx, blob.K8sPersistentVolumeName, metav1.GetOptions{})
	if err != nil {
		return
	}

	// get attachments annotation

	attachments, err := attachmentsFromK8sMeta(pv)
	if err != nil {
		return
	}

	// add attachment

	nodeWithDirectAttachment = attachments.Direct.Node

	if attachments.Direct.Node == node {
		attachments.Direct.Holders = slices.Remove(attachments.Direct.Holders, cookie)
	} else {
		i, indirectAtt := attachments.findIndirectAttachmentOn(node)

		indirectAtt.Holders = slices.Remove(indirectAtt.Holders, cookie)
		if len(indirectAtt.Holders) == 0 {
			indirectAttachmentsToCleanUp = append(indirectAttachmentsToCleanUp, indirectAtt.Node)
			attachments.Indirect = slices.RemoveAt(attachments.Indirect, i)
		}
	}

	if len(attachments.Direct.Holders) == 0 && len(attachments.Indirect) == 0 {
		shouldCleanUpDirectAttachment = true
	}

	return
}

func (bm *BlobManager) unregisterAttachmentOnK8sPvAnnotation(
	ctx context.Context, blob *Blob, node string, cookie string,
) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	err := bm.atomicUpdateK8sPvForBlob(ctx, blob, func(pv *corev1.PersistentVolume) error {
		// get attachments annotation

		attachments, err := attachmentsFromK8sMeta(pv)
		if err != nil {
			return err
		}

		// add attachment

		if attachments.Direct.Node == node {
			attachments.Direct.Holders = slices.Remove(attachments.Direct.Holders, cookie)
		} else {
			i, indirectAtt := attachments.findIndirectAttachmentOn(node)

			indirectAtt.Holders = slices.Remove(indirectAtt.Holders, cookie)
			if len(indirectAtt.Holders) == 0 {
				attachments.Indirect = slices.RemoveAt(attachments.Indirect, i)
			}
		}

		if len(attachments.Direct.Holders) == 0 && len(attachments.Indirect) == 0 {
			attachments.Direct = nil
		}

		// update attachments annotation

		err = attachments.setOnK8sMeta(pv)
		if err != nil {
			return err
		}

		return nil
	})

	return err
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
