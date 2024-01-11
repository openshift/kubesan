// SPDX-License-Identifier: Apache-2.0

package volumemanager

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"strings"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/config"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/lvm"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/nbd"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type attachments struct {
	Direct   *attachment  `json:"direct"` // nil if not attached anywhere
	Indirect []attachment `json:"indirect"`
}

type attachment struct {
	Node    string   `json:"node"`
	Holders []string `json:"holders"`
}

func (a *attachments) findIndirectAttachmentOn(node string) (int, *attachment) {
	for i, indirectAtt := range a.Indirect {
		if indirectAtt.Node == node {
			return i, &indirectAtt
		}
	}
	return -1, nil
}

// You must pass the same cookie to DetachVolume() when you no longer need the volume attached. The cookie is used
// internally to keep track of whether there's still someone who needs the volume attached.
//
// If node is nil, attach to any node.
//
// This method is idempotent and may be called from any node.
func (vm *VolumeManager) AttachVolume(
	ctx context.Context, vol *Volume, node *string, cookie string,
) (actualNode string, path string, err error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// update K8s PV annotation listing attachments

	nodeWithDirectAttachment, err := vm.registerAttachmentOnK8sPvAnnotation(ctx, vol, node, cookie)
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
		path, err = vm.attachDirect(ctx, vol, actualNode)
	} else {
		path, err = vm.attachIndirect(ctx, vol, actualNode, nodeWithDirectAttachment)
	}

	return
}

func (vm *VolumeManager) AttachVolumeUnmanaged(
	ctx context.Context, vol *Volume, node *string,
) (actualNode string, path string, err error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// attach the volume

	if node != nil {
		actualNode = *node
	} else {
		actualNode = config.LocalNodeName
	}

	path, err = vm.attachDirect(ctx, vol, actualNode)

	return
}

// If node is nil, chooses any node, kinda.
func (vm *VolumeManager) registerAttachmentOnK8sPvAnnotation(
	ctx context.Context, vol *Volume, node *string, cookie string,
) (nodeWithDirectAttachment string, err error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: vol.K8sPvName}}

	err = k8s.AtomicUpdate(
		ctx, vm.clientset.CoreV1().RESTClient(), "persistentvolumes", pv,
		func(pv *corev1.PersistentVolume) error {
			if pv.Annotations == nil {
				pv.Annotations = map[string]string{}
			}

			// get attachments annotation

			atts := attachments{}

			if attsString, ok := pv.Annotations[config.Domain+"/attachments"]; ok {
				err := json.Unmarshal([]byte(attsString), &atts)
				if err != nil {
					return err
				}
			}

			// add attachment

			if node == nil {
				if atts.Direct == nil {
					atts.Direct = &attachment{
						Node:    config.LocalNodeName,
						Holders: []string{cookie},
					}
				} else {
					atts.Direct.Holders = insert(atts.Direct.Holders, cookie)
				}
			} else {
				if atts.Direct == nil {
					atts.Direct = &attachment{
						Node:    *node,
						Holders: []string{cookie},
					}
				} else if atts.Direct.Node == *node {
					atts.Direct.Holders = insert(atts.Direct.Holders, cookie)
				} else if _, indirectAtt := atts.findIndirectAttachmentOn(*node); indirectAtt != nil {
					indirectAtt.Holders = insert(indirectAtt.Holders, cookie)
				} else {
					atts.Indirect = append(atts.Indirect, attachment{
						Node:    *node,
						Holders: []string{cookie},
					})
				}
			}

			nodeWithDirectAttachment = atts.Direct.Node

			// update attachments annotation

			if atts.Direct == nil && len(atts.Indirect) == 0 {
				delete(pv.Annotations, config.Domain+"/attachments")
			} else {
				attsBytes, err := json.Marshal(&atts)
				if err != nil {
					return err
				}

				pv.Annotations[config.Domain+"/attachments"] = string(attsBytes)
			}

			return nil
		},
	)
	if err != nil {
		err = status.Errorf(codes.Internal, "failed to set PV annotation: %s", err)
	}

	return
}

func (vm *VolumeManager) attachDirect(ctx context.Context, vol *Volume, node string) (string, error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// activate LVM thin pool and thin LVs

	hash := sha1.New()
	hash.Write([]byte(node))
	hash.Write([]byte(vol.K8sPvcUid))

	job := &jobs.Job{
		Name:     fmt.Sprintf("activate-lv-%x", hash.Sum(nil)),
		NodeName: node,
		Command: []string{
			"./lvm/activate.sh", vol.BackingDevicePath, vol.lvmThinPoolLvRef(), vol.lvmThinLvRef(),
		},
	}

	err := jobs.Run(ctx, vm.clientset, job)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to activate LVM LVs: %s", err)
	}

	err = jobs.Delete(ctx, vm.clientset, job.Name)
	if err != nil {
		return "", err
	}

	// get path to LVM thin LV

	lvmThinLvPath, err := vm.getVolumeLvmThinLvPath(ctx, vol)
	if err != nil {
		return "", err
	}

	// success

	return lvmThinLvPath, nil
}

func (vm *VolumeManager) attachIndirect(
	ctx context.Context, vol *Volume, node string, nodeWithDirectAttachment string,
) (string, error) {
	// TODO: Make this idempotent and concurrency-safe and ensure cleanup under error conditions.

	// start NBD server

	lvmThinLvPath, err := vm.getVolumeLvmThinLvPath(ctx, vol)
	if err != nil {
		return "", err
	}

	nbdServerId := nbd.ServerId{
		NodeName: nodeWithDirectAttachment,
		PvcUid:   vol.K8sPvcUid,
	}

	err = nbd.StartServer(ctx, vm.clientset, &nbdServerId, lvmThinLvPath)
	if err != nil {
		return "", err
	}

	// set up NBD client

	nbdDevicePath, err := nbd.ConnectClient(ctx, vm.clientset, node, &nbdServerId)
	if err != nil {
		return "", err
	}

	// success

	return nbdDevicePath, nil
}

// The returned path is valid on all nodes in the cluster.
//
// This method may be called from any node, and fails if the volume does not exist.
func (vm *VolumeManager) getVolumeLvmThinLvPath(ctx context.Context, vol *Volume) (string, error) {
	output, err := lvm.Command(
		ctx,
		"lvs",
		"--devices", vol.BackingDevicePath,
		"--options", "lv_path",
		"--noheadings",
		vol.lvmThinLvRef(),
	)
	if err != nil {
		return "", status.Errorf(codes.Internal, "failed to get path to LVM LV: %s: %s", err, output)
	}

	return strings.TrimSpace(output), nil
}

func insert[T comparable](list []T, elem T) []T {
	for _, e := range list {
		if e == elem {
			return list
		}
	}
	return append(list, elem)
}
