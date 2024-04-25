# SPDX-License-Identifier: Apache-2.0

# Usage: __stage <format> <args...>
__stage() {
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

# Usage: __poll <retry_delay> <max_tries> <command>
__poll() {
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

# Usage: __pod_is_running [-n=<pod_namespace>] <pod_name>
__pod_is_running() {
    [[ "$( kubectl get pod "$@" -o=jsonpath='{.status.phase}' )" = Running ]]
}

# Usage: __wait_for_pod_to_succeed <timeout_seconds> [-n=<pod_namespace>] <pod_name>
__wait_for_pod_to_succeed() {
    __poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Succeeded|Failed$ ]]"
    # shellcheck disable=SC2048,SC2086
    [[ "$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )" = Succeeded ]]
}

# Usage: __wait_for_pod_to_start_running <timeout_seconds> [-n=<pod_namespace>] <pod_name>
__wait_for_pod_to_start_running() {
    __poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Running|Succeeded|Failed$ ]]"
}

# Usage: __wait_for_pvc_to_be_bound <timeout_seconds> [-n=<pvc_namespace>] <pvc_name>
__wait_for_pvc_to_be_bound() {
    __poll 1 "$1" "[[ \"\$( kubectl get pvc ${*:2} -o=jsonpath='{.status.phase}' )\" = Bound ]]"
}

# Usage: __wait_for_vs_to_be_ready <timeout_seconds> [-n=<vs_namespace>] <vs_name>
__wait_for_vs_to_be_ready() {
    __poll 1 "$1" "[[ \"\$( kubectl get vs ${*:2} -o=jsonpath='{.status.readyToUse}' )\" = true ]]"
}

# Usage: __create_volume <name> <size>
__create_volume() {
    name=$1
    size=$2

    __stage "Creating volume \"$name\"..."

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

    __wait_for_pvc_to_be_bound 300 "$name"
}

# Usage: __fill_volume <name> <size_mb>
__fill_volume() {
    name=$1
    size_mb=$2

    __stage "Writing random data to volume \"$name\"..."

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

    __wait_for_pod_to_succeed 60 test-pod
    kubectl delete pod test-pod --timeout=60s
}

# Usage: __create_snapshot <volume> <snapshot>
__create_snapshot() {
    volume=$1
    snapshot=$2

    __stage "Snapshotting \"$volume\"..."

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

    __wait_for_vs_to_be_ready 60 "$snapshot"
}
