# SPDX-License-Identifier: Apache-2.0

ksan-supported-modes Linear Thin

ksan-create-fs-volume test-pvc 64Mi

ksan-stage 'Creating files as root...'

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
      touch /mnt/test &&
      mkdir /mnt/my-dir &&
      touch /mnt/my-dir/foo
    volumeMounts:
    - name: pvc
      mountPath: /mnt
  volumes:
  - name: pvc
    persistentVolumeClaim:
      claimName: test-pvc
EOF

ksan-wait-for-pod-to-succeed 60 "test-pod"

kubectl delete pod test-pod --timeout=30s

ksan-stage 'Mounting volume with fsGroup...'

kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  terminationGracePeriodSeconds: 0
  restartPolicy: Never
  securityContext:
    fsGroup: 107
  containers:
  - name: container
    image: $TEST_IMAGE
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
        - ALL
      runAsNonRoot: true
      runAsUser: 107
      runAsGroup: 107
      seccompProfile:
        type: RuntimeDefault
    command:
    - bash
    - -c
    - |
      test -G /mnt &&
      test -G /mnt/test &&
      test -G /mnt/my-dir &&
      test -G /mnt/my-dir/foo
    volumeMounts:
    - name: pvc
      mountPath: /mnt
  volumes:
  - name: pvc
    persistentVolumeClaim:
      claimName: test-pvc
EOF

ksan-wait-for-pod-to-succeed 60 "test-pod"

kubectl delete pod test-pod --timeout=30s

ksan-delete-volume test-pvc

ksan-stage 'Finishing test...'
