# SPDX-License-Identifier: Apache-2.0

sp-stage 'Provisioning %d volumes...' "${#NODES[@]}"

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
EOF
done

for i in "${!NODES[@]}"; do
    sp-wait-for-pvc-to-be-bound 300 "test-pvc-$i"
done

sp-stage 'Mounting each volume read-write on a different node...'

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
    sp-wait-for-pod-to-start-running 60 "test-pod-$i"
done

sleep 10

for i in "${!NODES[@]}"; do
    sp-pod-is-running "test-pod-$i"
done

sp-stage 'Unmounting volumes...'

kubectl delete pod "${NODE_INDICES[@]/#/test-pod-}" --timeout=30s

sp-stage 'Deleting volumes...'

kubectl delete pvc "${NODE_INDICES[@]/#/test-pvc-}" --timeout=30s
