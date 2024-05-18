# SPDX-License-Identifier: Apache-2.0

# Usage: __sp-get-node-name <node_name>|<node_index>
__sp-get-node-name() {
    if [[ ! "$1" =~ ^[0-9]+$ ]]; then
        echo "$1"
    elif (( "$1" == 0 )); then
        # shellcheck disable=SC2154
        echo "${current_cluster}"
    else
        printf '%s-m%02d' "${current_cluster}" "$(( $1 + 1 ))"
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

# Usage: sp-ssh-into-node <node_name>|<node_index> [<command...>]
sp-ssh-into-node() {
    if (( $# < 1 )); then
        >&2 echo "Usage: sp-ssh-into-node <node_name>|<node_index> [<args...>]"
        return 2
    elif (( $# == 1 )); then
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __sp-get-node-name "$1" )" \
            -- \
            bash -i
    else
        local __args="${*:2}"
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __sp-get-node-name "$1" )" \
            -- \
            bash -ic "${__args@Q}" bash
    fi
}
export -f sp-ssh-into-node

sp-retry() {
    # shellcheck disable=SC2154
    touch "${subprovisioner_retry_path}"
    exit 0
}
export -f sp-retry
