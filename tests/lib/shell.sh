#!/bin/bash
# SPDX-License-Identifier: Apache-2.0

__shell() {
    __log "$1" 'Starting interactive shell.'
    __log "$1" 'Inspect the cluster with:'
    __log "$1" '  $ kubectl [...]'
    __log "$1" '  $ ksan-csi-controller-plugin describe|exec|logs [<args...>]'
    __log "$1" '  $ ksan-csi-node-plugin <node_name>|<node_index> describe|exec|logs [<args...>]'
    __log "$1" '  $ ksan-ssh-into-node <node_name>|<node_index> [<command...>]'

    if [[ "$2" == true ]]; then
        __log "$1" 'To reset the sandbox:'
        __log "$1" '  $ ksan-retry'
    else
        __log "$1" 'To retry the current test with a new cluster:'
        __log "$1" '  $ ksan-retry'
        __log "$1" 'To cancel this and all remaining tests:'
        __log "$1" '  $ ksan-cancel'
    fi
    __log "$1" 'To load rebuilt images in current cluster (assumes no API/YAML changes):'
    __log "$1" '  $ ksan-reimage'

    IFS='/' read -r -a script_path <<< "$0"

    if [[ "${script_path[0]}" == "" ]]; then
        # absolute path
        script_path=( "/${script_path[1]}" "${script_path[@]:2}" )
    fi

    if (( ${#script_path[@]} > 2 )); then
        script_path=( ... "${script_path[@]: -2}" )
    fi

    kubesan_tests_run_sh_path=$( printf '/%s' "${script_path[@]}" )
    kubesan_tests_run_sh_path=${kubesan_tests_run_sh_path:1}

    (
        export kubesan_tests_run_sh_path
        export kubesan_retry_path="${temp_dir}/retry"
        export kubesan_cancel_path="${temp_dir}/cancel"
        cd "${initial_working_dir}"
        # shellcheck disable=SC2016,SC2028
        "$BASH" --init-file <( echo "
            . \"\$HOME/.bashrc\"
            . \"${repo_root}/tests/lib/logs.sh\"
            PROMPT_COMMAND=(
                \"echo -en '\\001\\033[1m\\002(\$kubesan_tests_run_sh_path)\\001\\033[0m\\002 '\"
                \"\${PROMPT_COMMAND[@]}\"
                )
            " )
    ) || true
}
