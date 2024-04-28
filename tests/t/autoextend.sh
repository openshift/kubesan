# SPDX-License-Identifier: Apache-2.0
#
# This test verifies that the thin pool grows automatically when a volume is
# overwritten while snapshots still reference old blocks. Worst case data
# consumption occurs when a snapshot is created and the volume is subsequently
# completely overwritten.

__create_volume test-pvc-1 64Mi

__fill_volume test-pvc-1 64
__create_snapshot test-pvc-1 test-vs-1

__fill_volume test-pvc-1 64
__create_snapshot test-pvc-1 test-vs-2

__fill_volume test-pvc-1 64

__stage 'Deleting volume 1...'
kubectl delete pvc test-pvc-1 --timeout=60s

__stage 'Deleting first snapshot of volume 1...'
kubectl delete vs test-vs-1 --timeout=60s

__stage 'Deleting second snapshot of volume 1...'
kubectl delete vs test-vs-2 --timeout=60s
