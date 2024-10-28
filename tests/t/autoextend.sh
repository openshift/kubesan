# SPDX-License-Identifier: Apache-2.0
#
# This test verifies that the thin pool grows automatically when a volume is
# overwritten while snapshots still reference old blocks. Worst case data
# consumption occurs when a snapshot is created and the volume is subsequently
# completely overwritten.

ksan-supported-modes # TODO enable Thin when snapshots are implemented

ksan-create-volume test-pvc-1 64Mi

ksan-fill-volume test-pvc-1 64
ksan-create-snapshot test-pvc-1 test-vs-1

ksan-fill-volume test-pvc-1 64
ksan-create-snapshot test-pvc-1 test-vs-2

ksan-fill-volume test-pvc-1 64

ksan-delete volume test-pvc-1

ksan-stage 'Deleting first snapshot of volume 1...'
kubectl delete vs test-vs-1 --timeout=60s

ksan-stage 'Deleting second snapshot of volume 1...'
kubectl delete vs test-vs-2 --timeout=60s

ksan-stage 'Finishing test...'
