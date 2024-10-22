# SPDX-License-Identifier: Apache-2.0

# This test does not use ksan-supported-modes because it directly tests the
# NbdExport CRD without using Volumes or StorageClass at all.

ksan-stage "Creating NbdExport..."

kubectl create -f - <<EOF
apiVersion: kubesan.gitlab.io/v1alpha1
kind: NbdExport
metadata:
  name: export
  namespace: kubesan-system
spec:
  source: "/dev/null"
  host: $(__ksan-get-node-name 0)
EOF

# Wait for Status.Conditions["Available"]
ksan-poll 1 30 "kubectl get --namespace kubesan-system -o=jsonpath='{.status.conditions[*]['\''type'\'','\''status'\'']}' nbdexport export | grep --quiet 'Available True'"

# TODO Connect a temporary NBD client to the export, to prove that CR
# creation does trigger the correct access.

ksan-stage "Adding client..."
kubectl patch --namespace kubesan-system nbdexport export --type merge -p "
spec:
  clients:
    - $(__ksan-get-node-name 1)
"

ksan-stage "Deleting export..."
kubectl delete --namespace kubesan-system --wait=false nbdexport export
ksan-poll 1 30 "kubectl get --namespace kubesan-system -o=jsonpath='{.status.conditions[*]['\''type'\'','\''status'\'']}' nbdexport export | grep --quiet 'Available False'"

ksan-stage "Dropping client..."
kubectl patch --namespace kubesan-system nbdexport export --type merge -p "
spec:
  clients: []
"
ksan-poll 1 30 "[[ -z \"\$(kubectl get --no-headers --namespace kubesan-system nbdexport 2>/dev/null)\" ]]"

ksan-stage "Cleaning up..."
