// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	blobs "gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/blobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/controller"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/identity"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/node"
	"google.golang.org/grpc"
)

func RunControllerPlugin(csiSocketPath string) error {
	listener, server, err := setup(csiSocketPath)
	if err != nil {
		return err
	}

	blobManager, err := blobs.NewBlobManager()
	if err != nil {
		return err
	}

	// run gRPC server

	csi.RegisterIdentityServer(server, &identity.IdentityServer{})
	csi.RegisterControllerServer(server, &controller.ControllerServer{BlobManager: blobManager})
	return server.Serve(listener)

	// TODO: Handle SIGTERM gracefully.
}

func RunNodePlugin(csiSocketPath string) error {
	listener, server, err := setup(csiSocketPath)
	if err != nil {
		return err
	}

	blobManager, err := blobs.NewBlobManager()
	if err != nil {
		return err
	}

	// run gRPC server

	csi.RegisterIdentityServer(server, &identity.IdentityServer{})
	csi.RegisterNodeServer(server, &node.NodeServer{BlobManager: blobManager})
	return server.Serve(listener)

	// TODO: Handle SIGTERM gracefully.
}

func setup(csiSocketPath string) (net.Listener, *grpc.Server, error) {
	// create gRPC server

	err := os.Remove(csiSocketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}

	listener, err := net.Listen("unix", csiSocketPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen: %v", err)
	}

	loggingInterceptor := func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		log.Printf("%s({ %+v})", info.FullMethod, req)
		resp, err := handler(ctx, req)
		if err == nil {
			log.Printf("%s(...) --> { %+v}", info.FullMethod, resp)
		} else {
			log.Printf("%s(...) --> %+v", info.FullMethod, err)
		}
		return resp, err
	}

	contextInterceptor := func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// The default context often has a too-short deadline, interrupting out work midway and later retrying
		// it just for it to get interrupted midway once again. Work around this simply by disabling any
		// timeout, ensuring we make progress. Note that Kubernetes will still eventually retry the call,
		// potentially concurrently to the current call. Hopefully there is some limit to the number of gRPCs
		// that this server can process at once, and this doesn't cause a thousand coroutines to run at once.
		ctx = context.Background()

		return handler(ctx, req)
	}

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(loggingInterceptor, contextInterceptor))

	return listener, server, nil
}
