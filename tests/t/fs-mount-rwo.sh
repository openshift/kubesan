# SPDX-License-Identifier: Apache-2.0

ksan-stage 'Provisioning %d volumes...' "${#NODES[@]}"

for i in "${!NODES[@]}"; do
    ksan-create-fs-volume test-pvc-$i 64Mi
done

ksan-stage 'Mounting each volume read-write on a different node...'

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
              dd if=/dev/zero of=/mnt/pvc/test-file bs=1M count=32 oflag=direct &&
              sleep infinity
          volumeMounts:
            - name: pvc
              mountPath: /mnt/pvc
        - name: mkdir-container
          image: $TEST_IMAGE
          command:
            - bash
            - -c
            - |
              while true; do
                mkdir /mnt/pvc/test-dir;
                touch /mnt/pvc/test-dir/a;
                rm -rf /mnt/pvc/test-dir;
              done
          volumeMounts:
            - name: pvc
              mountPath: /mnt/pvc
      volumes:
        - name: pvc
          persistentVolumeClaim:
            claimName: test-pvc-$i
EOF
done

for i in "${!NODES[@]}"; do
    ksan-wait-for-pod-to-start-running 60 "test-pod-$i"
done

sleep 10

for i in "${!NODES[@]}"; do
    ksan-pod-is-running "test-pod-$i"
done

ksan-stage 'Unmounting volumes...'

kubectl delete pod "${NODE_INDICES[@]/#/test-pod-}" --timeout=30s

ksan-delete-volume "${NODE_INDICES[@]/#/test-pvc-}"

ksan-stage 'Finishing test...'
