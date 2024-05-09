# SPDX-License-Identifier: Apache-2.0

# Usage: __sp-get-node-name <node_name>|<node_index>
__sp-get-node-name() {
    if [[ ! "$1" =~ ^[0-9]+$ ]]; then
        echo "$1"
    elif [[ -z "${REPO_ROOT:-}" ]]; then
        # not being run under subprovisioner test script
        local __nodes
        # shellcheck disable=SC2207
        __nodes=( $( kubectl get nodes --output=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' ) ) &&
        (( $1 < ${#__nodes[@]} )) &&
        echo "${__nodes[$1]}"
    elif (( "$1" == 0 )); then
        # shellcheck disable=SC2154
        echo "${current_cluster}"
    else
        printf '%s-m%02d\n' "${current_cluster}" "$(( $1 + 1 ))"
    fi
}
export -f __sp-get-node-name

# Usage: __sp-get-pod-name <component> [<node>]
__sp-get-pod-name() {
    kubectl get pod \
        --namespace subprovisioner \
        --selector "subprovisioner.gitlab.io/component==$1" \
        ${2+"--field-selector=spec.nodeName==$( __sp-get-node-name "$2" )"} \
        --output jsonpath="{.items[0].metadata.name}"
}
export -f __sp-get-pod-name

# Usage: __sp-per-node-component <caller> <component> <node_name>|<node_index> describe|exec|logs [<args...>]
__sp-per-node-component() {
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
        --namespace subprovisioner \
        "${__kubectl_cmd[@]}" \
        "$( __sp-get-pod-name "$2" "$3" )" \
        "${__kubectl_args[@]}"
}
export -f __sp-per-node-component

# Usage: sp-csi-controller-plugin describe|exec|logs [<args...>]
sp-csi-controller-plugin() {
    if (( $# < 1 )); then
        >&2 echo "Usage: sp-csi-controller-plugin describe|exec|logs [<args...>]"
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
        >&2 echo "Usage: sp-csi-controller-plugin describe|exec|logs [<args...>]"
        return 2
        ;;
    esac

    kubectl \
        --namespace subprovisioner \
        "${__kubectl_cmd[@]}" \
        "$( __sp-get-pod-name csi-controller-plugin )" \
        "${__kubectl_args[@]}"
}
export -f sp-csi-controller-plugin

# Usage: sp-csi-node-plugin <node_name>|<node_index> describe|exec|logs [<args...>]
sp-csi-node-plugin() {
    __sp-per-node-component sp-csi-node-plugin csi-node-plugin "$@"
}
export -f sp-csi-node-plugin

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
export -f sp-poll

# Usage: sp-pod-is-running [-n=<pod_namespace>] <pod_name>
sp-pod-is-running() {
    [[ "$( kubectl get pod "$@" -o=jsonpath='{.status.phase}' )" = Running ]]
}
export -f sp-pod-is-running

# Usage: sp-wait-for-pod-to-succeed <timeout_seconds> [-n=<pod_namespace>] <pod_name>
sp-wait-for-pod-to-succeed() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Succeeded|Failed$ ]]"
    # shellcheck disable=SC2048,SC2086
    [[ "$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )" = Succeeded ]]
}
export -f sp-wait-for-pod-to-succeed

# Usage: sp-wait-for-pod-to-start-running <timeout_seconds> [-n=<pod_namespace>] <pod_name>
sp-wait-for-pod-to-start-running() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get pod ${*:2} -o=jsonpath='{.status.phase}' )\" =~ ^Running|Succeeded|Failed$ ]]"
}
export -f sp-wait-for-pod-to-start-running

# Usage: sp-wait-for-pvc-to-be-bound <timeout_seconds> [-n=<pvc_namespace>] <pvc_name>
sp-wait-for-pvc-to-be-bound() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get pvc ${*:2} -o=jsonpath='{.status.phase}' )\" = Bound ]]"
}
export -f sp-wait-for-pvc-to-be-bound

# Usage: sp-wait-for-vs-to-be-bound <timeout_seconds> [-n=<vs_namespace>] <vs_name>
sp-wait-for-vs-to-be-bound() {
    sp-poll 1 "$1" "[[ \"\$( kubectl get vs ${*:2} -o=jsonpath='{.status.readyToUse}' )\" = true ]]"
}
export -f sp-wait-for-vs-to-be-bound
