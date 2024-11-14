// SPDX-License-Identifier: Apache-2.0

package dm

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"gitlab.com/kubesan/kubesan/internal/common/commands"
	"gitlab.com/kubesan/kubesan/internal/common/config"
)

// This package provides the code for idempotent manipulation of
// device mapper wrappers for thin volumes using dmsetup in the host
// namespace.  Note that we need two devices per Volume.  This is
// because "dmsetup suspend/dmsetup resume" is the only way to
// live-swap which underlying block device is dereferenced, however,
// any userspace application doing IO to a dm device that is suspended
// will block in D state until the resume.  Meanwhile, the only device
// mapper object that can queue I/O without blocking the userspace
// client is multipath (even if we are only using a single path),
// using "dmsetup message" to fail or reinstate the underlying path as
// a faster way than waiting for the underlying storage to block.
// Since we don't want userspace to block the upper layer has to be a
// dm-multipath device that never suspends, but that means it can
// never hot-swap the underlying device, so we also need a lower layer
// dm-linear object that can hot-swap between direct LV access or
// /dev/nbdX NBD client access.

// Create the wrappers in the filesystem so that the device can be opened;
// however, I/O to the device is not possible until Resume() is used.
func Create(ctx context.Context, name string, sizeBytes int64) error {
	log := log.FromContext(ctx).WithValues("nodeName", config.LocalNodeName)

	// Use of --notable instead of zeroTable() is not portable to all
	// versions of device-mapper.
	_, err := commands.DmsetupCreateIdempotent(lowerName(name), "--table", zeroTable(sizeBytes), "--addnodeoncreate")
	if err != nil {
		log.Error(err, "dm lower create failed")
		return err
	}

	// At least with device-mapper 1.02.199, we have to avoid udev to
	// not hang, at which point not even --addnodeoncreate will create
	// a /dev/mapper entry without a separate mknodes call.  Thankfully,
	// our use of the device does not depend on udev.
	_, err = commands.DmsetupCreateIdempotent(upperName(name), "--table", upperTable(sizeBytes, name), "--noudevsync")
	if err != nil {
		log.Error(err, "dm upper create failed")
		_, _ = commands.DmsetupRemoveIdempotent(lowerName(name))
		return err
	}

	_, err = commands.Dmsetup("mknodes", upperName(name))
	if err != nil {
		log.Error(err, "dm upper mknodes failed")
		_ = Remove(ctx, name)
		return err
	}

	// Sanity check that the filesystem devices exist in /dev/mapper now,
	// useful since dmsetup does not accept / in its device names.
	exists, err := commands.PathExistsOnHost(GetDevicePath(name))
	if err == nil && !exists {
		err = errors.NewBadRequest("missing expected dm mapper path")
	}
	if err != nil {
		log.Error(err, "dm mknode failed")
		_ = Remove(ctx, name)
		return err
	}

	return nil
}

// Suspend the device by queuing I/O until the next Resume.  Do not try
// to sync any filesystem if the device is block storage.
func Suspend(ctx context.Context, name string, skipSync bool) error {
	log := log.FromContext(ctx).WithValues("nodeName", config.LocalNodeName)

	if exists, err := commands.PathExistsOnHost(GetDevicePath(name)); err == nil && exists {
		_, err := commands.Dmsetup("message", upperName(name), "0", "fail_path /dev/mapper/"+lowerName(name))
		if err != nil {
			log.Error(err, "dm upper suspend failed")
			return err
		}

		args := []string{lowerName(name)}
		if skipSync {
			args = append(args, "--nolockfs")
		}
		_, err = commands.DmsetupSuspendIdempotent(args...)
		if err != nil {
			log.Error(err, "dm lower suspend failed")
			return err
		}
	}

	return nil
}

// Resume I/O on the volume, as routed through devPath.
func Resume(ctx context.Context, name string, sizeBytes int64, devPath string) error {
	log := log.FromContext(ctx).WithValues("nodeName", config.LocalNodeName)

	_, err := commands.Dmsetup("load", lowerName(name), "--table", lowerTable(sizeBytes, devPath))
	if err != nil {
		log.Error(err, "dm lower load failed")
		return err
	}

	_, err = commands.Dmsetup("resume", lowerName(name))
	if err != nil {
		log.Error(err, "dm lower resume failed")
		return err
	}

	_, err = commands.Dmsetup("message", upperName(name), "0", "reinstate_path /dev/mapper/"+lowerName(name))
	if err != nil {
		log.Error(err, "dm upper resume failed")
		return err
	}

	return nil
}

// Tear down the wrappers.  Should only be called when the device is not in use.
func Remove(ctx context.Context, name string) error {
	log := log.FromContext(ctx).WithValues("nodeName", config.LocalNodeName)

	// --force is necessary to make udev see EIO instead of hanging
	_, err := commands.DmsetupRemoveIdempotent("--force", upperName(name))
	if err != nil {
		log.Error(err, "dm upper remove failed")
		return err
	}

	_, err = commands.DmsetupRemoveIdempotent(lowerName(name))
	if err != nil {
		log.Error(err, "dm lower remove failed")
		return err
	}

	return nil
}

func GetDevicePath(name string) string {
	return "/dev/mapper/" + upperName(name)
}

func lowerName(name string) string {
	return name + "-dm-linear"
}

func upperName(name string) string {
	return name + "-dm-multi"
}

func zeroTable(sizeBytes int64) string {
	return fmt.Sprintf("0 %d zero", sizeBytes/512)
}

func lowerTable(sizeBytes int64, device string) string {
	return fmt.Sprintf("0 %d linear %s 0", sizeBytes/512, device)
}

func upperTable(sizeBytes int64, name string) string {
	return fmt.Sprintf("0 %d multipath 3 queue_if_no_path queue_mode bio 0 1 1 round-robin 0 1 0 /dev/mapper/%s", sizeBytes/512, lowerName(name))
}
