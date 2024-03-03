# SPDX-License-Identifier: Apache-2.0

# SOME DEFINITIONS

# Usage: start_pod <one_based_node_index>
start_pod() {
    local pod_name=test-pod-$1
    local node_name=${NODES[$1 - 1]}

    kubectl create -f - <<EOF
    apiVersion: v1
    kind: Pod
    metadata:
      name: $pod_name
    spec:
      nodeName: $node_name
      restartPolicy: Never
      containers:
        - name: container
          image: $TEST_IMAGE
          command:
            - fio
            - --name=global
            - --rw=randwrite
            - --fsync=1
            - --runtime=60m
            - --time_based=1
            - --filename=/var/pvc
            - --allow_file_create=0
            - --name=job1
          volumeDevices:
            - name: pvc
              devicePath: /var/pvc
      volumes:
        - name: pvc
          persistentVolumeClaim:
            claimName: test-pvc
EOF

    __wait_for_pod_to_start_running 60 "$pod_name"
}

# Usage: ensure_pod_is_writing <one_based_node_index>
ensure_pod_is_writing() {
    local pod_name=test-pod-$1

    sleep 10
    __pod_is_running "$pod_name"
}

# ACTUAL TEST

__stage 'Provisioning volume...'
kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  volumeMode: Block
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 64Mi
EOF
__wait_for_pvc_to_be_bound 300 test-pvc

__stage 'Launching pod mounting the volume and writing to it...'
start_pod 1
ensure_pod_is_writing 1

__stage 'Launching another pod on a different node mounting the volume and writing to it...'
start_pod 2
ensure_pod_is_writing 2

__stage 'Ensuring that the first pod is still writing to the volume...'
ensure_pod_is_writing 1

__stage 'Deleting the first pod...'
kubectl delete pod test-pod-1 --timeout=30s

__stage 'Ensuring that the second pod is still writing to the volume...'
ensure_pod_is_writing 2

__stage 'Deleting the second pod...'
kubectl delete pod test-pod-2 --timeout=30s

__stage 'Deleting volume...'
kubectl delete pvc test-pvc --timeout=30s
