#!/bin/bash

# Bindicate pre-pivot hook
# This script runs before the root filesystem is switched to set up bind mounts

type getarg >/dev/null 2>&1 || . /lib/dracut-lib.sh

# Check if bindicate should be disabled
if getargbool 0 rd.bindicate.disable; then
    info "Bindicate disabled via kernel parameter"
    exit 0
fi

# Get configuration file path (default or from kernel parameter)
BINDICATE_CONFIG=$(getarg rd.bindicate.config)
if [ -z "$BINDICATE_CONFIG" ]; then
    BINDICATE_CONFIG="/var/lib/bindicate/config.json"
fi

# Check if configuration file exists
if [ ! -f "$BINDICATE_CONFIG" ]; then
    warn "Bindicate configuration file not found: $BINDICATE_CONFIG"
    exit 0
fi

# Set up the sysroot prefix for bind mounts
# In initramfs context, we need to work with the mounted root filesystem
ROOT_MOUNT_POINT="$NEWROOT"

if [ ! -d "$ROOT_MOUNT_POINT" ]; then
    warn "Root filesystem not yet mounted, skipping bindicate"
    exit 0
fi

info "Running bindicate setup-bindmounts with config: $BINDICATE_CONFIG"

# Run bindicate to set up bind mounts
# We need to chroot into the new root to execute bindicate properly
if ! chroot "$ROOT_MOUNT_POINT" /usr/bin/bindicate "$BINDICATE_CONFIG" setup-bindmounts; then
    warn "Bindicate setup-bindmounts failed"
    exit 1
fi

info "Bindicate bind mounts setup completed successfully"

# Optionally update fstab for persistent mounts across reboots
if getargbool 0 rd.bindicate.update_fstab; then
    info "Updating fstab with bindicate entries"
    if ! chroot "$ROOT_MOUNT_POINT" /usr/bin/bindicate "$BINDICATE_CONFIG" write-fstab; then
        warn "Bindicate fstab update failed"
    else
        info "Bindicate fstab updated successfully"
    fi
fi