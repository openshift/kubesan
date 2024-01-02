# SPDX-License-Identifier: Apache-2.0

__stage 'Provisioning %d volumes...' "${#NODES[@]}"

for i in "${!NODES[@]}"; do
    kubectl create -f - <<EOF
    apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: test-pvc-$i
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: 64Mi
      volumeMode: Block
      storageClassName: storage-class
EOF
done

for i in "${!NODES[@]}"; do
    __wait_for_pvc_to_be_bound 30 "test-pvc-$i"
done

__stage 'Mounting each volume read-write on a different node...'

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
            claimName: test-pvc-$i
EOF
done

for i in "${!NODES[@]}"; do
    __wait_for_pod_to_start_running 30 "test-pod-$i"
done

sleep 10

for i in "${!NODES[@]}"; do
    __pod_is_running "test-pod-$i"
done

__stage 'Unmounting volumes...'

kubectl delete pod "${NODE_INDICES[@]/#/test-pod-}" --timeout=30s

__stage 'Deleting volumes...'

kubectl delete pvc "${NODE_INDICES[@]/#/test-pvc-}" --timeout=30s
