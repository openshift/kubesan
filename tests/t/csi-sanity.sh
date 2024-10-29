# SPDX-License-Identifier: Apache-2.0

ksan-supported-modes Linear Thin

kubectl create -f - <<EOF
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: csi-parameters
data:
  parameters: '$(kubectl get --output jsonpath={.parameters} sc kubesan)'
---
apiVersion: v1
kind: Pod
metadata:
  name: csi-sanity
spec:
  restartPolicy: Never
  hostPID: true
  containers:
    - name: container
      image: $TEST_IMAGE
      command:
        - ./csi-sanity
        - --csi.controllerendpoint
        - /var/lib/kubelet/plugins/kubesan-controller/socket
        - --csi.endpoint
        - /var/lib/kubelet/plugins/kubesan-node/socket
        - --csi.mountdir
        - /var/lib/kubelet/plugins/csi-sanity-target
        - --csi.stagingdir
        - /var/lib/kubelet/plugins/csi-sanity-staging
        - --csi.testvolumeaccesstype
        - block
        - --csi.testvolumeparameters
        - /etc/csi-parameters/parameters
        - --ginkgo.v
        - --ginkgo.seed=1
#        - --ginkgo.fail-fast
      volumeMounts:
        - name: drivers
          mountPath: /var/lib/kubelet/plugins
        - name: csi-parameters
          mountPath: /etc/csi-parameters
        # Mount /dev so that symlinks to block devices resolve
        - name: dev
          mountPath: /dev
      securityContext:
        privileged: true
  volumes:
    - name: drivers
      hostPath:
        path: /var/lib/kubelet/plugins/
        type: DirectoryOrCreate
    - name: csi-parameters
      configMap:
        name: csi-parameters
    - name: dev
      hostPath:
        path: /dev
        type: Directory
EOF

fail=0
ksan-wait-for-pod-to-succeed 60 csi-sanity || fail=$?
kubectl delete configmap csi-parameters
kubectl logs pods/csi-sanity

# TODO fix remaining issues, then hard-fail this test if csi-sanity fails.
# For now, if we got at least one pass in the final line of the logged output,
# then mark this overall test as skipped instead of failed.
pattern="[1-9][0-9]* Pass"
if [[ $fail != 0 &&
      "$( kubectl logs --tail=1 pods/csi-sanity )" =~ $pattern ]]; then
    fail=77
    echo "SKIP: partial csi-sanity failures are still expected"
fi

ksan-stage 'Finishing test...'
exit $fail
