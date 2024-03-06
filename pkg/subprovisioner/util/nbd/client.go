// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"context"
	"fmt"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/util"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/kubernetes"
)

func ConnectClient(ctx context.Context, clientset kubernetes.Interface, clientNode string, serverId *ServerId) (string, error) {
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
		Command:     []string{"scripts/nbd.sh", "client-connect", deviceSymlinkPath, serverIp.String()},
		HostNetwork: true, // for netlink to work
	}

	err = jobs.CreateAndRunAndDelete(ctx, clientset, job)
	if err != nil {
		return "", status.Errorf(
			codes.Internal,
			"failed to connect NBD client device to hostname '%s': %s",
			serverId.Hostname(), err,
		)
	}

	// success

	return deviceSymlinkPath, nil
}

func DisconnectClient(ctx context.Context, clientset kubernetes.Interface, clientNode string, serverId *ServerId) error {
	deviceSymlinkPath := fmt.Sprintf("/run/subprovisioner/nbd/%s", serverId.Hostname())

	job := &jobs.Job{
		Name:        fmt.Sprintf("nbd-disconnect-%s", util.Hash(clientNode, serverId.hash())),
		NodeName:    clientNode,
		Command:     []string{"scripts/nbd.sh", "client-disconnect", deviceSymlinkPath},
		HostNetwork: true, // for netlink to work
	}

	err := jobs.CreateAndRunAndDelete(ctx, clientset, job)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to disconnect NBD client device: %s", err)
	}

	return nil
}
