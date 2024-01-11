// SPDX-License-Identifier: Apache-2.0

package nbd

import (
	"context"
	"crypto/sha1"
	"fmt"

	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/jobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ConnectClient(ctx context.Context, clientset *k8s.Clientset, clientNode string, id *ServerId) (string, error) {
	// TODO: Make idempotent.
	// TODO: Find available device programmatically.

	// connect device to server

	hash := sha1.New()
	hash.Write([]byte(clientNode))
	hash.Write([]byte(id.NodeName))
	hash.Write([]byte(id.PvcUid))
	hashBytes := hash.Sum(nil)

	deviceSymlinkPath := fmt.Sprintf("/run/subprovisioner/nbd/%x", hashBytes)

	// we run the job in the host net namespace, so we must resolve the server's hostname here
	serverIp, err := id.ResolveHost()
	if err != nil {
		return "", err
	}

	job := &jobs.Job{
		Name:        fmt.Sprintf("nbd-connect-%x", hashBytes),
		NodeName:    clientNode,
		Command:     []string{"./nbd/connect.sh", deviceSymlinkPath, serverIp.String()},
		HostNetwork: true, // for netlink to work
	}

	err = jobs.Run(ctx, clientset, job)
	if err != nil {
		return "", status.Errorf(
			codes.Internal,
			"failed to connect NBD client device to hostname '%s': %s",
			id.Hostname(), err,
		)
	}

	err = jobs.Delete(ctx, clientset, job.Name)
	if err != nil {
		return "", err
	}

	// success

	return deviceSymlinkPath, nil
}

func DisconnectClient(ctx context.Context, clientset *k8s.Clientset, clientNode string, id *ServerId) error {
	hash := sha1.New()
	hash.Write([]byte(clientNode))
	hash.Write([]byte(id.NodeName))
	hash.Write([]byte(id.PvcUid))
	hashBytes := hash.Sum(nil)

	deviceSymlinkPath := fmt.Sprintf("/run/subprovisioner/nbd/%x", hashBytes)

	job := &jobs.Job{
		Name:        fmt.Sprintf("nbd-disconnect-%x", hashBytes),
		NodeName:    clientNode,
		Command:     []string{"./nbd/disconnect.sh", deviceSymlinkPath},
		HostNetwork: true, // for netlink to work
	}

	err := jobs.Run(ctx, clientset, job)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to disconnect NBD client device: %s", err)
	}

	err = jobs.Delete(ctx, clientset, job.Name)
	if err != nil {
		return err
	}

	return nil
}
