# SPDX-License-Identifier: Apache-2.0

__stage 'Provisioning volume...'

kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 64Mi
  volumeMode: Block
EOF

__wait_for_pvc_to_be_bound 60 test-pvc

__stage 'Mounting volume read-write on all nodes...'

for i in "${!NODES[@]}"; do
    kubectl create -f - <<EOF
    apiVersion: v1
    kind: Pod
    metadata:
      name: test-pod-$i
    spec:
      nodeName: ${NODES[i]}
      terminationGracePeriodSeconds: 0
      restartPolicy: Never
      containers:
        - name: container
          image: $TEST_IMAGE
          command:
            - bash
            - -c
            - |
              dd if=/dev/zero of=/var/pvc bs=1M count=64 oflag=direct &&
              sleep infinity
          volumeDevices:
            - name: pvc
              devicePath: /var/pvc
      volumes:
        - name: pvc
          persistentVolumeClaim:
            claimName: test-pvc
EOF
done

for i in "${!NODES[@]}"; do
    __wait_for_pod_to_start_running 60 "test-pod-$i"
done

sleep 10

for i in "${!NODES[@]}"; do
    __pod_is_running "test-pod-$i"
done

__stage 'Unmounting volume from all nodes...'

kubectl delete pod "${NODE_INDICES[@]/#/test-pod-}" --timeout=30s

__stage 'Deleting volume...'

kubectl delete pvc test-pvc --timeout=30s
