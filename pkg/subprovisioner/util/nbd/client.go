// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"context"
	"fmt"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ConnectClient(ctx context.Context, clientset *k8s.Clientset, clientNode string, serverId *ServerId) (string, error) {
	// TODO: Make idempotent.
	// TODO: Find available device programmatically.

	// connect device to server

	deviceSymlinkPath := fmt.Sprintf("/run/subprovisioner/nbd/%s", serverId.Hostname())

	// we run the job in the host net namespace, so we must resolve the server's hostname here
	serverIp, err := serverId.ResolveHost()
	if err != nil {
		return "", err
	}

	job := &jobs.Job{
		Name:        fmt.Sprintf("nbd-connect-%s", util.Hash(clientNode, serverId.hash())),
		NodeName:    clientNode,
		Command:     []string{"./nbd/client-connect.sh", deviceSymlinkPath, serverIp.String()},
		HostNetwork: true, // for netlink to work
	}

	err = jobs.CreateAndRun(ctx, clientset, job)
	if err != nil {
		return "", status.Errorf(
			codes.Internal,
			"failed to connect NBD client device to hostname '%s': %s",
			serverId.Hostname(), err,
		)
	}

	err = jobs.Delete(ctx, clientset, job.Name)
	if err != nil {
		return "", err
	}

	// success

	return deviceSymlinkPath, nil
}

func DisconnectClient(ctx context.Context, clientset *k8s.Clientset, clientNode string, serverId *ServerId) error {
	deviceSymlinkPath := fmt.Sprintf("/run/subprovisioner/nbd/%s", serverId.Hostname())

	job := &jobs.Job{
		Name:        fmt.Sprintf("nbd-disconnect-%s", util.Hash(clientNode, serverId.hash())),
		NodeName:    clientNode,
		Command:     []string{"./nbd/client-disconnect.sh", deviceSymlinkPath},
		HostNetwork: true, // for netlink to work
	}

	err := jobs.CreateAndRun(ctx, clientset, job)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to disconnect NBD client device: %s", err)
	}

	err = jobs.Delete(ctx, clientset, job.Name)
	if err != nil {
		return err
	}

	return nil
}
