// SPDX-License-Identifier: Apache-2.0

package cluster

import "context"

// An error indicating that a Watch is waiting and progress cannot be made
// until Reconcile() is called again.
//
// BlobManager methods do not block when waiting for Kubernetes resources.
// Instead they return WatchPending errors so the caller can handle other work
// in the meantime. When this error is returned, controllers should return
// success from Reconcile() since the runtime will invoke Reconcile() again
// when the Watch triggers.
type WatchPending struct{}

func (w *WatchPending) Error() string {
	return "watch pending"
}

// BlobManager abstracts operations that depend on the volume mode (linear or
// thin).
type BlobManager interface {
	// CreateBlob creates an empty blob of the given size if it does not
	// exist yet.
	//
	// If the blob already exists but the size does not match then it will
	// be recreated with the desired size.
	CreateBlob(ctx context.Context, name string, sizeBytes int64) error

	// RemoveBlob removes a blob if it exists. No error is returned if the
	// blob does not exist.
	RemoveBlob(ctx context.Context, name string) error
}
