# SPDX-License-Identifier: Apache-2.0

# Usage: __ksan-get-pod-name <component> [<node>]
__ksan-get-pod-name() {
    kubectl get pod \
        --namespace kubesan-system \
        --selector "app.kubernetes.io/component==$1" \
        ${2+"--field-selector=spec.nodeName==$( __ksan-get-node-name "$2" )"} \
        --output jsonpath="{.items[0].metadata.name}"
}
export -f __ksan-get-pod-name

# Usage: __ksan-get-node-name <node_name>|<node_index>
__ksan-get-node-name() {
    if [[ ! "$1" =~ ^[0-9]+$ ]]; then
        echo "$1"
    else
        local __nodes
        # shellcheck disable=SC2207
        __nodes=( $( kubectl get nodes --output=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' ) ) &&
        (( $1 < ${#__nodes[@]} )) &&
        echo "${__nodes[$1]}"
    fi
}
export -f __ksan-get-node-name

# Usage: __ksan-per-node-component <caller> <component> <node_name>|<node_index> describe|exec|logs [<args...>]
__ksan-per-node-component() {
    if (( $# < 4 )); then
        >&2 echo "Usage: $1 <node_name>|<node_index> describe|exec|logs [<args...>]"
        return 2
    fi

    local __kubectl_cmd __kubectl_args
    __kubectl_cmd=( "$4" )
    __kubectl_args=( "${@:5}" )

    case "$4" in
    describe)
        __kubectl_cmd+=( pod )
        ;;
    exec)
        if (( ${#__kubectl_args[@]} == 0 )); then
            __kubectl_args=( -it -- bash )
        fi
        ;;
    logs)
        ;;
    *)
        >&2 echo "Usage: $1 <node_name>|<node_index> describe|exec|logs [<args...>]"
        return 2
        ;;
    esac

    kubectl \
        --namespace kubesan-system \
        "${__kubectl_cmd[@]}" \
        "$( __ksan-get-pod-name "$2" "$3" )" \
        "${__kubectl_args[@]}"
}
export -f __ksan-per-node-component

# Usage: ksan-csi-controller-plugin describe|exec|logs [<args...>]
ksan-csi-controller-plugin() {
    if (( $# < 1 )); then
        >&2 echo "Usage: ksan-csi-controller-plugin describe|exec|logs [<args...>]"
        return 2
    fi

    local __kubectl_cmd __kubectl_args
    __kubectl_cmd=( "$1" )
    __kubectl_args=( "${@:2}" )

    case "$1" in
    describe)
        __kubectl_cmd+=( pod )
        ;;
    exec)
        if (( ${#__kubectl_args[@]} == 0 )); then
            __kubectl_args=( -it -- bash )
        fi
        ;;
    logs)
        ;;
    *)
        >&2 echo "Usage: ksan-csi-controller-plugin describe|exec|logs [<args...>]"
        return 2
        ;;
    esac

    kubectl \
        --namespace kubesan-system \
        "${__kubectl_cmd[@]}" \
        "$( __ksan-get-pod-name csi-controller-plugin )" \
        "${__kubectl_args[@]}"
}
export -f ksan-csi-controller-plugin

# Usage: ksan-csi-node-plugin <node_name>|<node_index> describe|exec|logs [<args...>]
ksan-csi-node-plugin() {
    __ksan-per-node-component ksan-csi-node-plugin csi-node-plugin "$@"
}
export -f ksan-csi-node-plugin

# Usage: ksan-poll <retry_delay> <max_tries> <command>
ksan-poll() {
    (
        set -o errexit -o pipefail -o nounset +o xtrace

        for (( i = 1; $2 == 0 || i < $2; ++i )); do
            if eval "${*:3}"; then return 0; fi
            sleep "$1"
        done

        if eval "${*:3}"; then return 0; fi

        return 1
    )
}
export -f ksan-poll

# Usage: ksan-pod-is-running [-n=<pod_namespace>] <pod_name>
ksan-pod-is-running() {
    [[ "$( kubectl get pod "$@" -o=jsonpath='{.status.phase}' )" = Running ]]
}
export -f ksan-pod-is-running

# Usage: ksan-wait-for-pod-to-succeed <timeout_seconds> [-n=<pod_namespace>] <pod_name>
ksan-wait-for-pod-to-succeed() {
    ksan-poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Succeeded|Failed$ ]]"
    # shellcheck disable=SC2048,SC2086
    [[ "$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )" = Succeeded ]]
}
export -f ksan-wait-for-pod-to-succeed

# Usage: ksan-wait-for-pod-to-start-running <timeout_seconds> [-n=<pod_namespace>] <pod_name>
ksan-wait-for-pod-to-start-running() {
    ksan-poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Running|Succeeded|Failed$ ]]"
}
export -f ksan-wait-for-pod-to-start-running

# Usage: ksan-wait-for-pvc-to-be-bound <timeout_seconds> [-n=<pvc_namespace>] <pvc_name>
ksan-wait-for-pvc-to-be-bound() {
    ksan-poll 1 "$1" "[[ \"\$( kubectl get pvc ${*:2} -o=jsonpath='{.status.phase}' )\" = Bound ]]"
}
export -f ksan-wait-for-pvc-to-be-bound

# Usage: ksan-wait-for-vs-to-be-bound <timeout_seconds> [-n=<vs_namespace>] <vs_name>
ksan-wait-for-vs-to-be-bound() {
    ksan-poll 1 "$1" "[[ \"\$( kubectl get vs ${*:2} -o=jsonpath='{.status.readyToUse}' )\" = true ]]"
}
export -f ksan-wait-for-vs-to-be-bound

if (( ${sandbox:-1} )); then

    # Usage: ksan-pull <images...>
    ksan-pull() {
        if (( $# == 0 )); then
            >&2 echo "Usage: ksan-pull <images...>"
            return 2
        fi

        local __containers="" __i __name="kubesan-pre-pull-images" __desired

        for (( __i = 1; __i <= $#; ++__i )); do
            __containers+=", { name: c$__i, image: \"${!__i}\", command: [ \"true\" ] }"
        done

        kubectl create -f - <<-EOF || return 1
        apiVersion: apps/v1
        kind: DaemonSet
        metadata:
          name: $__name
        spec:
          selector:
            matchLabels: &labels
              app.kubernetes.io/component: $__name
          template:
            metadata:
              labels: *labels
            spec:
              initContainers: [ ${__containers:2} ]
              containers:
                - name: pause
                  image: gcr.io/google_containers/pause:3.2
EOF

        __desired=$( kubectl get daemonset "$__name" -o=jsonpath='{.status.desiredNumberScheduled}' )

        echo "Waiting for image(s) to be pulled in $__desired nodes..."

        ksan-poll 1 0 \
            "(( \$( kubectl get daemonset \"$__name\" -o=jsonpath='{.status.numberReady}' )
            == $__desired )) " \
        || {
            kubectl delete daemonset "$__name"
            return 1
        }

        echo "Done."

        kubectl delete daemonset "$__name"
    }
    export -f ksan-pull

fi
