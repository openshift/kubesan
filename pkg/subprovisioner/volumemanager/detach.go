// SPDX-License-Identifier: Apache-2.0

package volumemanager

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/nbd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// This method is idempotent and may be called from any node.
func (vm *VolumeManager) DetachVolume(ctx context.Context, vol *Volume, node string, cookie string) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// update K8s PV annotation listing attachments

	nodeWithDirectAttachment, directAttachmentToCleanUp, indirectAttachmentsToCleanUp, err :=
		vm.unregisterAttachmentOnK8sPvAnnotation(ctx, vol, node, cookie)
	if err != nil {
		return err
	}

	// detach the volume

	for _, node := range indirectAttachmentsToCleanUp {
		err := vm.detachIndirect(ctx, vol, node, nodeWithDirectAttachment)
		if err != nil {
			return err
		}
	}

	if directAttachmentToCleanUp != nil {
		err := vm.detachDirect(ctx, vol, *directAttachmentToCleanUp)
		if err != nil {
			return err
		}
	}

	// success

	return nil
}

// This method is idempotent and may be called from any node.
func (vm *VolumeManager) DetachVolumeUnmanaged(ctx context.Context, vol *Volume, node string) error {
	return vm.detachDirect(ctx, vol, node)
}

// If node is nil, chooses any node, kinda.
func (vm *VolumeManager) unregisterAttachmentOnK8sPvAnnotation(
	ctx context.Context, vol *Volume, node string, cookie string,
) (
	nodeWithDirectAttachment string,
	directAttachmentToCleanUp *string,
	indirectAttachmentsToCleanUp []string,
	err error,
) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: vol.K8sPvName}}

	err = k8s.AtomicUpdate(
		ctx, vm.clientset.CoreV1().RESTClient(), "persistentvolumes", pv,
		func(pv *corev1.PersistentVolume) error {
			if pv.Annotations == nil {
				pv.Annotations = map[string]string{}
			}

			// get attachments annotation

			attsString, _ := pv.Annotations[config.Domain+"/attachments"]
			atts := attachments{}

			err := json.Unmarshal([]byte(attsString), &atts)
			if err != nil {
				return err
			}

			// add attachment

			// TODO: Don't add duplicate cookies.

			nodeWithDirectAttachment = atts.Direct.Node

			if atts.Direct.Node == node {
				atts.Direct.Holders = remove(atts.Direct.Holders, cookie)
			} else {
				i, indirectAtt := atts.findIndirectAttachmentOn(node)

				indirectAtt.Holders = remove(indirectAtt.Holders, cookie)
				if len(indirectAtt.Holders) == 0 {
					indirectAttachmentsToCleanUp = append(indirectAttachmentsToCleanUp, indirectAtt.Node)
					atts.Indirect = removeAt(atts.Indirect, i)
				}
			}

			if len(atts.Direct.Holders) == 0 && len(atts.Indirect) == 0 {
				directAttachmentToCleanUp = &atts.Direct.Node
				atts.Direct = nil
			}

			// update attachments annotation

			attsBytes, err := json.Marshal(&atts)
			if err != nil {
				return err
			}

			pv.Annotations[config.Domain+"/attachments"] = string(attsBytes)

			return nil
		},
	)
	if err != nil {
		err = status.Errorf(codes.Internal, "failed to set PV annotation: %s", err)
	}

	return
}

func (vm *VolumeManager) detachDirect(ctx context.Context, vol *Volume, node string) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// stop NBD server (which may have been started when creating an indirect attachment)

	nbdServerId := nbd.ServerId{
		NodeName: node,
		PvcUid:   vol.K8sPvcUid,
	}

	err := nbd.StopServer(ctx, vm.clientset, &nbdServerId)
	if err != nil {
		return err
	}

	// deactivate LVM thin and thin pool LVs

	hash := sha1.New()
	hash.Write([]byte(node))
	hash.Write([]byte(vol.K8sPvcUid))

	job := &jobs.Job{
		Name:     fmt.Sprintf("deactivate-lv-%x", hash.Sum(nil)),
		NodeName: node,
		Command: []string{
			"./lvm/deactivate.sh", vol.BackingDevicePath, vol.lvmThinLvRef(), vol.lvmThinPoolLvRef(),
		},
	}

	err = jobs.Run(ctx, vm.clientset, job)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to deactivate LVM LVs: %s", err)
	}

	err = jobs.Delete(ctx, vm.clientset, job.Name)
	if err != nil {
		return err
	}

	// success

	return nil
}

func (vm *VolumeManager) detachIndirect(
	ctx context.Context, vol *Volume, node string, nodeWithDirectAttachment string,
) error {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// tear down NBD client

	nbdServerId := nbd.ServerId{
		NodeName: nodeWithDirectAttachment,
		PvcUid:   vol.K8sPvcUid,
	}

	err := nbd.DisconnectClient(ctx, vm.clientset, node, &nbdServerId)
	if err != nil {
		return err
	}

	// success

	return nil
}

func remove[T comparable](list []T, elem T) []T {
	for i, e := range list {
		if e == elem {
			return removeAt(list, i)
		}
	}
	return list
}

func removeAt[T any](list []T, index int) []T {
	list[index] = list[len(list)-1]
	return list[:len(list)-1]
}
