// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubesan/kubesan/api/v1alpha1"
	"github.com/kubesan/kubesan/internal/common/config"
)

func (s *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	// validate request

	if req.SourceVolumeId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify source volume id")
	}

	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify snapshot name")
	}

	// create snapshot

	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: config.Namespace,
		},
		Spec: v1alpha1.SnapshotSpec{
			SourceVolume: req.SourceVolumeId,
		},
	}

	if err := s.client.Create(ctx, snapshot); err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	err := s.client.WatchSnapshotUntil(ctx, snapshot, func() bool {
		return conditionsv1.IsStatusConditionTrue(snapshot.Status.Conditions, conditionsv1.ConditionAvailable)
	})
	if err != nil {
		return nil, err
	}

	condition := conditionsv1.FindStatusCondition(snapshot.Status.Conditions, conditionsv1.ConditionAvailable)

	resp := &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SizeBytes:      *snapshot.Status.SizeBytes,
			SnapshotId:     snapshot.Name,
			SourceVolumeId: snapshot.Spec.SourceVolume,
			CreationTime:   timestamppb.New(condition.LastTransitionTime.Time),
			ReadyToUse:     true,
		},
	}

	return resp, nil
}

func (s *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// validate request

	if req.SnapshotId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "must specify snapshot id")
	}

	// delete snapshot

	snapshot := &v1alpha1.Snapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.SnapshotId,
			Namespace: config.Namespace,
		},
	}

	propagation := client.PropagationPolicy(metav1.DeletePropagationForeground)

	if err := s.client.Delete(ctx, snapshot, propagation); err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	// success

	resp := &csi.DeleteSnapshotResponse{}

	return resp, nil
}
