# SPDX-License-Identifier: Apache-2.0

__create_volume test-pvc-1 64Mi
__fill_volume test-pvc-1 64

__stage 'Creating volume 2 by cloning volume 1...'

kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc-2
spec:
  volumeMode: Block
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 64Mi
  dataSource:
    kind: PersistentVolumeClaim
    name: test-pvc-1
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
kubectl delete pod test-pod --timeout=30s

__stage 'Deleting volume 1...'

kubectl delete pvc test-pvc-1 --timeout=30s

__stage 'Creating volume 3 by cloning volume 2 but with a bigger size...'

kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc-3
spec:
  volumeMode: Block
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 128Mi
  dataSource:
    kind: PersistentVolumeClaim
    name: test-pvc-2
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
          cmp -n "${mib64}" /var/pvc-2 /var/pvc-3
          cmp -n "${mib64}" /var/pvc-3 /dev/zero "${mib64}"
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
kubectl delete pod test-pod --timeout=30s

__stage 'Deleting volumes 2 and 3...'

kubectl delete pvc test-pvc-2 test-pvc-3 --timeout=30s
