// SPDX-License-Identifier: Apache-2.0

package csi

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/external-snapshotter/client/v6/clientset/versioned"
	blobs "gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/blobs"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/controller"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/identity"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/csi/node"
	"gitlab.com/subprovisioner/subprovisioner/pkg/subprovisioner/util/k8s"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func RunControllerPlugin(csiSocketPath string) error {
	clientset, listener, server, err := setup(csiSocketPath)
	if err != nil {
		return err
	}

	// run gRPC server

	csi.RegisterIdentityServer(server, &identity.IdentityServer{})
	csi.RegisterControllerServer(server, &controller.ControllerServer{
		Clientset:   clientset,
		BlobManager: blobs.NewBlobManager(clientset),
	})
	return server.Serve(listener)

	// TODO: Handle SIGTERM gracefully.
}

func RunNodePlugin(csiSocketPath string) error {
	clientset, listener, server, err := setup(csiSocketPath)
	if err != nil {
		return err
	}

	// run gRPC server

	csi.RegisterIdentityServer(server, &identity.IdentityServer{})
	csi.RegisterNodeServer(server, &node.NodeServer{
		Clientset:   clientset,
		BlobManager: blobs.NewBlobManager(clientset),
	})
	return server.Serve(listener)

	// TODO: Handle SIGTERM gracefully.
}

func setup(csiSocketPath string) (*k8s.Clientset, net.Listener, *grpc.Server, error) {
	// set up Kubernetes API connection

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, nil, err
	}

	kubernetesClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	snapshotClientset, err := versioned.NewForConfig(config)
	if err != nil {
		return nil, nil, nil, err
	}

	clientset := &k8s.Clientset{
		Clientset:         kubernetesClientset,
		SnapshotClientSet: snapshotClientset,
	}

	// create gRPC server

	err = os.Remove(csiSocketPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, nil, nil, err
	}

	listener, err := net.Listen("unix", csiSocketPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to listen: %v", err)
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

	return clientset, listener, server, nil
}
