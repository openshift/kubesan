// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc"

	"github.com/kubesan/kubesan/internal/common/config"
	csiclient "github.com/kubesan/kubesan/internal/csi/common/client"
	"github.com/kubesan/kubesan/internal/csi/controller"
	"github.com/kubesan/kubesan/internal/csi/identity"
	"github.com/kubesan/kubesan/internal/csi/node"
)

func RunControllerPlugin() error {
	return serve(func(server *grpc.Server, client *csiclient.CsiK8sClient) {
		csi.RegisterIdentityServer(server, &identity.IdentityServer{})
		csi.RegisterControllerServer(server, controller.NewControllerServer(client))
	})
}

func RunNodePlugin() error {
	return serve(func(server *grpc.Server, client *csiclient.CsiK8sClient) {
		csi.RegisterIdentityServer(server, &identity.IdentityServer{})
		csi.RegisterNodeServer(server, node.NewNodeServer(client))
	})
}

func serve(register func(*grpc.Server, *csiclient.CsiK8sClient)) error {
	// create Kubernetes client

	client, err := csiclient.NewCsiK8sClient()
	if err != nil {
		return err
	}

	// remove any leftover socket file

	err = os.Remove(config.CsiSocketPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// create gRPC server

	listener, err := net.Listen("unix", config.CsiSocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
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

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(loggingInterceptor))

	register(server, client)

	// run gRPC server

	return server.Serve(listener)

	// TODO: Handle SIGTERM gracefully.
}
