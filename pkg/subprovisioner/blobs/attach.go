// SPDX-License-Identifier: Apache-2.0

package blobs

import (
	"context"
	"encoding/json"
	"fmt"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/nbd"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/slices"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type attachment struct {
	Node    string   `json:"node"`
	Holders []string `json:"holders"`
}

type attachments struct {
	Direct   *attachment  `json:"direct"` // nil if not attached anywhere
	Indirect []attachment `json:"indirect"`
}

func (a *attachments) findIndirectAttachmentOn(node string) (int, *attachment) {
	for i, indirectAtt := range a.Indirect {
		if indirectAtt.Node == node {
			return i, &indirectAtt
		}
	}
	return -1, nil
}

func attachmentsFromK8sMeta(obj metav1.ObjectMetaAccessor) (*attachments, error) {
	if obj.GetObjectMeta().GetAnnotations() == nil {
		obj.GetObjectMeta().SetAnnotations(map[string]string{})
	}

	annotation, _ := obj.GetObjectMeta().GetAnnotations()[config.Domain+"/pool-attachments"]
	a := &attachments{Indirect: []attachment{}}

	if annotation != "" {
		err := json.Unmarshal([]byte(annotation), a)
		if err != nil {
			return nil, err
		}
	}

	return a, nil
}

func (a *attachments) setOnK8sMeta(obj metav1.ObjectMetaAccessor) error {
	if a.Indirect == nil {
		a.Indirect = []attachment{}
	}

	annotation, err := json.Marshal(a)
	if err != nil {
		return err
	}

	if obj.GetObjectMeta().GetAnnotations() == nil {
		obj.GetObjectMeta().SetAnnotations(map[string]string{})
	}

	obj.GetObjectMeta().GetAnnotations()[config.Domain+"/pool-attachments"] = string(annotation)

	return nil
}

// You must pass the same cookie to DetachBlob() when you no longer need the volume to be attached. The cookie is used
// internally to keep track of whether there's still someone who needs the volume attached.
//
// If node is nil, attach to any node.
//
// This method is idempotent and may be called from any node.
//
// (Internal behavior note: If node is nil, the resulting attachment is guaranteed to be direct.)
func (bm *BlobManager) AttachBlob(
	ctx context.Context, blob *Blob, node *string, cookie string,
) (actualNode string, path string, err error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// update K8s PV annotation listing attachments

	nodeWithDirectAttachment, err := bm.registerAttachmentOnK8sPvAnnotation(ctx, blob, node, cookie)
	if err != nil {
		return
	}

	// attach the volume

	if node != nil {
		actualNode = *node
	} else {
		actualNode = nodeWithDirectAttachment
	}

	if actualNode == nodeWithDirectAttachment {
		path, err = bm.attachDirect(ctx, blob, actualNode)
	} else {
		path, err = bm.attachIndirect(ctx, blob, actualNode, nodeWithDirectAttachment)
	}

	return
}

func (bm *BlobManager) AttachBlobUnmanaged(
	ctx context.Context, blob *Blob, node *string,
) (actualNode string, path string, err error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// attach the volume

	if node != nil {
		actualNode = *node
	} else {
		actualNode = config.LocalNodeName
	}

	path, err = bm.attachDirect(ctx, blob, actualNode)

	return
}

// If node is nil, chooses any node, kinda.
func (bm *BlobManager) registerAttachmentOnK8sPvAnnotation(
	ctx context.Context, blob *Blob, node *string, cookie string,
) (nodeWithDirectAttachment string, err error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	err = bm.atomicUpdateK8sPvForBlob(ctx, blob, func(pv *corev1.PersistentVolume) error {
		// get attachments annotation

		attachments, err := attachmentsFromK8sMeta(pv)
		if err != nil {
			return err
		}

		// add attachment

		if node == nil {
			if attachments.Direct == nil {
				attachments.Direct = &attachment{
					Node:    config.LocalNodeName,
					Holders: []string{cookie},
				}
			} else {
				attachments.Direct.Holders = slices.AppendUnique(attachments.Direct.Holders, cookie)
			}
		} else {
			if attachments.Direct == nil {
				attachments.Direct = &attachment{
					Node:    *node,
					Holders: []string{cookie},
				}
			} else if attachments.Direct.Node == *node {
				attachments.Direct.Holders = slices.AppendUnique(attachments.Direct.Holders, cookie)
			} else if _, indirectAtt := attachments.findIndirectAttachmentOn(*node); indirectAtt != nil {
				indirectAtt.Holders = slices.AppendUnique(indirectAtt.Holders, cookie)
			} else {
				attachments.Indirect = append(attachments.Indirect, attachment{
					Node:    *node,
					Holders: []string{cookie},
				})
			}
		}

		nodeWithDirectAttachment = attachments.Direct.Node

		// update attachments annotation

		err = attachments.setOnK8sMeta(pv)
		if err != nil {
			return err
		}

		return nil
	})

	return
}

func (bm *BlobManager) attachDirect(ctx context.Context, blob *Blob, node string) (string, error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// activate LVM thin pool and thin LVs

	job := &jobs.Job{
		Name:     fmt.Sprintf("activate-lv-%s", util.Hash(node, blob.lvmThinLvRef())),
		NodeName: node,
		Command: []string{
			"./lvm/activate.sh", blob.BackingDevicePath, blob.lvmThinPoolLvRef(), blob.lvmThinLvRef(),
		},
	}

	err := jobs.CreateAndRun(ctx, bm.clientset, job)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to activate LVM LVs: %s", err)
	}

	err = jobs.Delete(ctx, bm.clientset, job.Name)
	if err != nil {
		return "", err
	}

	// get path to LVM thin LV

	lvmThinLvPath, err := bm.getBlobLvmThinLvPath(ctx, blob)
	if err != nil {
		return "", err
	}

	// success

	return lvmThinLvPath, nil
}

func (bm *BlobManager) attachIndirect(
	ctx context.Context, blob *Blob, node string, nodeWithDirectAttachment string,
) (string, error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// start NBD server

	lvmThinLvPath, err := bm.getBlobLvmThinLvPath(ctx, blob)
	if err != nil {
		return "", err
	}

	nbdServerId := &nbd.ServerId{
		NodeName: nodeWithDirectAttachment,
		BlobName: blob.Name,
	}

	err = nbd.StartServer(ctx, bm.clientset, nbdServerId, lvmThinLvPath)
	if err != nil {
		return "", err
	}

	// set up NBD client

	nbdDevicePath, err := nbd.ConnectClient(ctx, bm.clientset, node, nbdServerId)
	if err != nil {
		return "", err
	}

	// success

	return nbdDevicePath, nil
}
