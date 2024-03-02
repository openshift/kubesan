# SPDX-License-Identifier: Apache-2.0

__stage 'Creating volume 1...'

kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc-1
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 64Mi
  volumeMode: Block
EOF

__wait_for_pvc_to_be_bound 300 test-pvc-1

__stage 'Writing some random data to volume 1...'

kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  restartPolicy: Never
  containers:
    - name: container
      image: $TEST_IMAGE
      command:
        - bash
        - -c
        - |
          set -o errexit -o pipefail -o nounset -o xtrace
          dd if=/dev/urandom of=/var/pvc-1 conv=fsync bs=1M count=64
      volumeDevices:
        - { name: test-pvc-1, devicePath: /var/pvc-1 }
  volumes:
    - { name: test-pvc-1, persistentVolumeClaim: { claimName: test-pvc-1 } }
EOF

__wait_for_pod_to_succeed 60 test-pod
kubectl delete pod test-pod --timeout=60s

__stage 'Snapshotting volume 1...'

kubectl create -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: test-vs-1
spec:
  volumeSnapshotClassName: subprovisioner
  source:
    persistentVolumeClaimName: test-pvc-1
EOF

__wait_for_vs_to_be_ready 60 test-vs-1

__stage 'Deleting snapshot of volume 1...'

kubectl delete vs test-vs-1 --timeout=60s

__stage 'Snapshotting volume 1 again...'

kubectl create -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: test-vs-1
spec:
  volumeSnapshotClassName: subprovisioner
  source:
    persistentVolumeClaimName: test-pvc-1
EOF

__wait_for_vs_to_be_ready 60 test-vs-1

__stage 'Creating volume 2 from snapshot of volume 1...'

kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc-2
spec:
  volumeMode: Block
  dataSource:
    apiGroup: snapshot.storage.k8s.io
    kind: VolumeSnapshot
    name: test-vs-1
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 64Mi
EOF

__wait_for_pvc_to_be_bound 300 test-pvc-2

__stage 'Validating volume data and independence between volumes 1 and 2...'

kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  restartPolicy: Never
  containers:
    - name: container
      image: $TEST_IMAGE
      command:
        - bash
        - -c
        - |
          set -o errexit -o pipefail -o nounset -o xtrace
          cmp /var/pvc-1 /var/pvc-2
          dd if=/dev/urandom of=/var/pvc-2 conv=fsync bs=1M count=1
          ! cmp /var/pvc-1 /var/pvc-2
      volumeDevices:
        - { name: test-pvc-1, devicePath: /var/pvc-1 }
        - { name: test-pvc-2, devicePath: /var/pvc-2 }
  volumes:
    - { name: test-pvc-1, persistentVolumeClaim: { claimName: test-pvc-1 } }
    - { name: test-pvc-2, persistentVolumeClaim: { claimName: test-pvc-2 } }
EOF

__wait_for_pod_to_succeed 60 test-pod
kubectl delete pod test-pod --timeout=60s

# TODO: We can't yet enable this because we rely on the PV existing until all of
# its snapshots are gone. Reenable after we lift that restriction somehow.
# __stage 'Deleting volume 1...'
# kubectl delete pvc test-pvc-1 --timeout=60s

__stage 'Creating volume 3 from the snapshot of volume 1 but with a bigger size...'

kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc-3
spec:
  volumeMode: Block
  dataSource:
    apiGroup: snapshot.storage.k8s.io
    kind: VolumeSnapshot
    name: test-vs-1
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 128Mi
EOF

__wait_for_pvc_to_be_bound 300 test-pvc-3

__stage 'Validating volume data and independence between volumes 2 and 3...'

mib64="$(( 64 * 1024 * 1024 ))"

kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  restartPolicy: Never
  containers:
    - name: container
      image: $TEST_IMAGE
      command:
        - bash
        - -c
        - |
          set -o errexit -o pipefail -o nounset -o xtrace
          ! cmp -n "${mib64}" /var/pvc-2 /var/pvc-3
          cmp -n "${mib64}" /var/pvc-3 /dev/zero "${mib64}"
          dd if=/var/pvc-2 of=/var/pvc-3 conv=fsync bs=1M count=1
          cmp -n "${mib64}" /var/pvc-2 /var/pvc-3
          dd if=/dev/urandom of=/var/pvc-3 conv=fsync bs=1M count=1
          ! cmp -n "${mib64}" /var/pvc-2 /var/pvc-3
      volumeDevices:
        - { name: test-pvc-2, devicePath: /var/pvc-2 }
        - { name: test-pvc-3, devicePath: /var/pvc-3 }
  volumes:
    - { name: test-pvc-2, persistentVolumeClaim: { claimName: test-pvc-2 } }
    - { name: test-pvc-3, persistentVolumeClaim: { claimName: test-pvc-3 } }
EOF

__wait_for_pod_to_succeed 60 test-pod
kubectl delete pod test-pod --timeout=60s

__stage 'Deleting snapshot of volume 1...'

kubectl delete vs test-vs-1 --timeout=60s

__stage 'Deleting volumes 2 and 3...'

kubectl delete pvc test-pvc-2 test-pvc-3 --timeout=60s
