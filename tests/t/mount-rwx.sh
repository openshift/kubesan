# SPDX-License-Identifier: Apache-2.0

sp-stage 'Provisioning volumes...'

# Two distinct volumes, to ensure parallel cross-node NBD devices work

for i in 1 2; do
    kubectl create -f - <<EOF
    apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: test-pvc-$i
    spec:
      accessModes:
        - ReadWriteMany
      resources:
        requests:
          storage: $(( 64 * i ))Mi
      volumeMode: Block
EOF
done

sp-wait-for-pvc-to-be-bound 300 test-pvc-1
sp-wait-for-pvc-to-be-bound 300 test-pvc-2

sp-stage 'Mounting volumes read-write on all nodes...'

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
              dd if=/dev/zero of=/var/pvc1 bs=1M count=64 oflag=direct &&
              dd if=/dev/zero of=/var/pvc2 bs=1M count=128 oflag=direct &&
              sleep infinity
          volumeDevices:
            - name: pvc-1
              devicePath: /var/pvc1
            - name: pvc-2
              devicePath: /var/pvc2
      volumes:
        - name: pvc-1
          persistentVolumeClaim:
            claimName: test-pvc-1
        - name: pvc-2
          persistentVolumeClaim:
            claimName: test-pvc-2
EOF
done

for i in "${!NODES[@]}"; do
    sp-wait-for-pod-to-start-running 60 "test-pod-$i"
done

sleep 10

for i in "${!NODES[@]}"; do
    sp-pod-is-running "test-pod-$i"
done

sp-stage 'Unmounting volumes from all nodes...'

kubectl delete pod "${NODE_INDICES[@]/#/test-pod-}" --timeout=30s

sp-stage 'Deleting volumes...'

kubectl delete pvc test-pvc-1 test-pvc-2 --timeout=30s
