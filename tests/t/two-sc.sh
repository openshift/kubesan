# SPDX-License-Identifier: Apache-2.0

sp-stage 'Creating second shared VG'

__minikube_ssh "${NODES[0]}" "
    sudo lvm vgcreate --shared second-vg /dev/subprovisioner-drive-1
"

sp-stage 'Creating second StorageClass'

kubectl create -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: second
  annotations:
    storageclass.kubernetes.io/is-default-class: "false"
provisioner: subprovisioner.gitlab.io
parameters:
  backingVolumeGroup: second-vg
EOF

sp-stage 'Provisioning volumes in each StorageClass...'

# make_pvc sc_name
make_pvc()
{
    local sc_name="$1"

    kubectl create -f - <<EOF
    apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: test-pvc-${sc_name}
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 64Mi
      volumeMode: Block
      storageClassName: ${sc_name}
EOF
}

make_pvc subprovisioner
make_pvc second

sp-wait-for-pvc-to-be-bound 300 "test-pvc-subprovisioner"
sp-wait-for-pvc-to-be-bound 300 "test-pvc-second"

sp-stage 'Mounting both volumes read-write...'

kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  terminationGracePeriodSeconds: 0
  restartPolicy: Never
  containers:
    - name: container
      image: $TEST_IMAGE
      command:
        - bash
        - -c
        - |
          dd if=/var/pvc1 of=/var/pvc2 bs=1M count=64 oflag=direct &&
          sleep infinity
      volumeDevices:
        - name: pvc1
          devicePath: /var/pvc1
        - name: pvc2
          devicePath: /var/pvc2
  volumes:
    - name: pvc1
      persistentVolumeClaim:
        claimName: test-pvc-subprovisioner
    - name: pvc2
      persistentVolumeClaim:
        claimName: test-pvc-second
EOF

sp-wait-for-pod-to-start-running 60 "test-pod"
sp-pod-is-running "test-pod"

sp-stage 'Unmounting volumes...'

kubectl delete pod "test-pod" --timeout=30s

sp-stage 'Deleting volumes...'

kubectl delete pvc "test-pvc-subprovisioner" "test-pvc-second" --timeout=30s
