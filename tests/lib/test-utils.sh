# SPDX-License-Identifier: Apache-2.0

# Usage: sp-stage <format> <args...>
sp-stage() {
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        # shellcheck disable=SC2059
        text="$( printf "$@" )"
        text_lower="${text,,}"

        # shellcheck disable=SC2154
        if (( pause_on_stage )); then
            __log_yellow "Pausing before ${text_lower::1}${text:1}"
            __shell 32 false
        fi

        printf "\033[36m--- [%6.1f] %s\033[0m\n" "$( __elapsed )" "${text}"
    )
}

# Usage: sp-poll <retry_delay> <max_tries> <command>
sp-poll() {
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        for (( i = 1; i < "$2"; ++i )); do
            if eval "${*:3}"; then return 0; fi
            sleep "$1"
        done

        if eval "${*:3}"; then return 0; fi

        return 1
    )
}

# Usage: sp-pod-is-running [-n=<pod_namespace>] <pod_name>
sp-pod-is-running() {
    [[ "$( kubectl get pod "$@" -o=jsonpath='{.status.phase}' )" = Running ]]
}

# Usage: sp-wait-for-pod-to-succeed <timeout_seconds> [-n=<pod_namespace>] <pod_name>
sp-wait-for-pod-to-succeed() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Succeeded|Failed$ ]]"
    # shellcheck disable=SC2048,SC2086
    [[ "$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )" = Succeeded ]]
}

# Usage: sp-wait-for-pod-to-start-running <timeout_seconds> [-n=<pod_namespace>] <pod_name>
sp-wait-for-pod-to-start-running() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Running|Succeeded|Failed$ ]]"
}

# Usage: sp-wait-for-pvc-to-be-bound <timeout_seconds> [-n=<pvc_namespace>] <pvc_name>
sp-wait-for-pvc-to-be-bound() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get pvc ${*:2} -o=jsonpath='{.status.phase}' )\" = Bound ]]"
}

# Usage: sp-wait-for-vs-to-be-bound <timeout_seconds> [-n=<vs_namespace>] <vs_name>
sp-wait-for-vs-to-be-bound() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get vs ${*:2} -o=jsonpath='{.status.readyToUse}' )\" = true ]]"
}

# Usage: sp-create-volume <name> <size>
sp-create-volume() {
    name=$1
    size=$2

    sp-stage "Creating volume \"$name\"..."

    kubectl create -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $name
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: $size
  volumeMode: Block
EOF

    sp-wait-for-pvc-to-be-bound 300 "$name"
}

# Usage: sp-fill-volume <name> <size_mb>
sp-fill-volume() {
    name=$1
    size_mb=$2

    sp-stage "Writing random data to volume \"$name\"..."

    kubectl create -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  restartPolicy: Never
  containers:
    - name: container
      image: $TEST_IMAGE
      command:
        - bash
        - -c
        - |
          set -o errexit -o pipefail -o nounset -o xtrace
          dd if=/dev/urandom of=/var/pvc conv=fsync bs=1M count=$size_mb
      volumeDevices:
        - { name: $name, devicePath: /var/pvc }
  volumes:
    - { name: $name, persistentVolumeClaim: { claimName: $name } }
EOF

    sp-wait-for-pod-to-succeed 60 test-pod
    kubectl delete pod test-pod --timeout=60s
}

# Usage: sp-create-snapshot <volume> <snapshot>
sp-create-snapshot() {
    volume=$1
    snapshot=$2

    sp-stage "Snapshotting \"$volume\"..."

    kubectl create -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: $snapshot
spec:
  volumeSnapshotClassName: subprovisioner
  source:
    persistentVolumeClaimName: $volume
EOF

    sp-wait-for-vs-to-be-bound 60 "$snapshot"
}
