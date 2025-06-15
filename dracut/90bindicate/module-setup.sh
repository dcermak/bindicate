#!/bin/bash

# dracut module for bindicate
# This module includes the bindicate binary and configuration in the initramfs

check() {
    # Check if bindicate binary exists
    require_binaries bindicate || return 1
    return 0
}

depends() {
    # Dependencies - we need basic filesystem support
    echo rootfs-block
}

install() {
    # Install the bindicate binary
    inst_binary bindicate

    # Install configuration file if it exists
    if [[ -f /var/lib/bindicate/config.json ]]; then
        inst /var/lib/bindicate/config.json
    fi

    # Create the configuration directory in initramfs
    inst_dir /var/lib/bindicate

    # Install the bindicate service script
    inst_script "$moddir/bindicate.sh" /lib/dracut/hooks/pre-pivot/90-bindicate.sh

    # Install required libraries for JSON parsing and file operations
    inst_libdir_file "libc.so*"
}