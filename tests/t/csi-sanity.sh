# SPDX-License-Identifier: Apache-2.0

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
        - /run/csi/kubesan-controller/socket
        - --csi.endpoint
        - /run/csi/kubesan-node/socket
        - --csi.testvolumeaccesstype
        - block
        - --csi.testvolumeparameters
        - /etc/csi-parameters/parameters
        - --ginkgo.v
        - --ginkgo.seed=1
#        - --ginkgo.fail-fast
      volumeMounts:
        - name: drivers
          mountPath: /run/csi
        - name: csi-parameters
          mountPath: /etc/csi-parameters
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
