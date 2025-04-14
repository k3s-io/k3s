#!/bin/bash

# Usage: ./cleanup_vms.sh [regex_pattern]
# Default pattern matches timestamped VMs older than 2 hours.
# We embed the time in the VM name, so we can easily filter them out.

# Get the current time in seconds since the epoch
current_time=$(date +%s)


def_pattern="_([0-9]+)_(server|agent)"
if [ -n "$1" ]; then
    pattern="$1"
else
    pattern="$def_pattern"
fi

# Get the list of VMs
vms=$(virsh list --name --all)
# Cleanup running VMs, happens if a previous test panics
for vm in $vms; do
    if [[ $vm =~ $pattern ]]; then
        vm_time="${BASH_REMATCH[1]}"
        age=$((current_time - vm_time))
        if [ $age -gt 7200 ] || [ "$pattern" != "$def_pattern" ]; then
            virsh destroy $vm
            virsh undefine $vm --remove-all-storage
        fi
    fi
done

# Cleanup inactive domains, happens if previous test is canceled
vms=$(virsh list --name --inactive)
for vm in $vms; do
    if [[ $vm =~ $pattern ]]; then
        vm_time="${BASH_REMATCH[1]}"
        age=$((current_time - vm_time))
        if [ $age -gt 7200 ] || [ "$pattern" != "$def_pattern" ]; then
            virsh undefine $vm  --remove-all-storage
        fi
    fi
done