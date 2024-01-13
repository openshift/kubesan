# SPDX-License-Identifier: Apache-2.0

# Usage: __get_node_name <node_name>|<node_index>
__get_node_name() {
    if [[ ! "$1" =~ ^[0-9]+$ ]]; then
        echo "$1"
    elif (( "$1" == 0 )); then
        # shellcheck disable=SC2154
        echo "${current_cluster}"
    else
        printf '%s-m%02d' "${current_cluster}" "$(( $1 + 1 ))"
    fi
}
export -f __get_node_name

# Usage: __get_pod_name <component> [<node>]
__get_pod_name() {
    kubectl get pod \
        --namespace subprovisioner \
        --selector "subprovisioner.gitlab.io/component==$1" \
        ${2+"--field-selector=spec.nodeName==$( __get_node_name "$2" )"} \
        --output jsonpath="{.items[0].metadata.name}"
}
export -f __get_pod_name

# Usage: __controller_plugin describe|exec|logs [<args...>]
__controller_plugin() {
    if (( $# < 1 )); then
        >&2 echo "Usage: __controller_plugin describe|exec|logs [<args...>]"
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
        >&2 echo "Usage: __controller_plugin describe|exec|logs [<args...>]"
        return 2
        ;;
    esac

    kubectl \
        --namespace subprovisioner \
        "${__kubectl_cmd[@]}" \
        "$( __get_pod_name csi-controller-plugin )" \
        "${__kubectl_args[@]}"
}
export -f __controller_plugin

# Usage: __per_node_component <caller> <component> <node_name>|<node_index> describe|exec|logs [<args...>]
__per_node_component() {
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
        "$( __get_pod_name "$2" "$3" )" \
        "${__kubectl_args[@]}"
}
export -f __per_node_component

# Usage: __node_plugin <node_name>|<node_index> describe|exec|logs [<args...>]
__node_plugin() {
    __per_node_component __node_plugin csi-node-plugin "$@"
}
export -f __node_plugin

# Usage: __lvmlockd <node_name>|<node_index> describe|exec|logs [<args...>]
__lvmlockd() {
    __per_node_component __lvmlockd lvmlockd "$@"
}
export -f __lvmlockd

# Usage: __ssh_into_node <node_name>|<node_index> [<command...>]
__ssh_into_node() {
    if (( $# < 1 )); then
        >&2 echo "Usage: __ssh_into_node <node_name>|<node_index> [<args...>]"
        return 2
    elif (( $# == 1 )); then
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __get_node_name "$1" )" \
            -- \
            bash -i
    else
        local __args="${*:2}"
        minikube \
            --profile "${current_cluster}" \
            ssh \
            --node "$( __get_node_name "$1" )" \
            -- \
            bash -ic "${__args@Q}" bash
    fi
}
export -f __ssh_into_node
