# SPDX-License-Identifier: Apache-2.0

____run_in_test_container_aux() {
    local __opts=()

    while [[ "$1" == -* ]]; do
        while (( $# > 0 )); do
            if [[ "$1" == -- ]]; then
                shift
                break
            else
                __opts+=( "$1" )
                shift
            fi
        done
    done

    # nbd-client needs '--net host' for netlink
    docker run \
        --privileged \
        -v /dev:/dev -v /etc:/etc -v /run:/run -v /var:/var \
        "${__opts[@]}" \
        docker.io/localhost/subprovisioner/test:test \
        "$@"
}

__run_in_test_container() {
    ____run_in_test_container_aux -i --rm -- "$@"
}

__run_in_test_container_async() {
    ____run_in_test_container_aux -d -- "$@"
}
