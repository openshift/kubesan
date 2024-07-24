# SPDX-License-Identifier: Apache-2.0

# usage __create_ksan_sharedvg <vgname> <device>
__create_ksan_shared_vg() {
    __minikube_ssh "${NODES[0]}" "
        sudo vgcreate --shared "$1" "$2"
    "

    for node in "${NODES[@]}"; do
        __minikube_ssh "${node}" "
	sudo lvmdevices --devicesfile "$1" --adddev "$2"
        sudo vgchange --lockstart "$1"
        sudo vgimportdevices "$1" --devicesfile "$1"
        "
    done
}
export -f __create_ksan_shared_vg
