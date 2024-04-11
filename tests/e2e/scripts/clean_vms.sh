#!/bin/bash

# Clean up any VMS that are older than 2 hours.
# 
# We embed the time in the VM name, so we can easily filter them out.

# Get the current time in seconds since the epoch
current_time=$(date +%s)

# Get the list of VMs
vms=$(virsh list --name --all)
time_regex="_([0-9]+)_(server|agent)"
# Cleanup running VMs, happens if a previous test panics
for vm in $vms; do
    if [[ $vm =~ $time_regex ]]; then
        vm_time="${BASH_REMATCH[1]}"
        age=$((current_time - vm_time))
        if [ $age -gt 7200 ]; then
            virsh destroy $vm
            virsh undefine $vm --remove-all-storage
        fi
    fi
done

# Cleanup inactive domains, happens if previous test is canceled
vms=$(virsh list --name --inactive)
for vm in $vms; do
    if [[ $vm =~ $time_regex ]]; then
        vm_time="${BASH_REMATCH[1]}"
        age=$((current_time - vm_time))
        if [ $age -gt 7200 ]; then
            virsh undefine $vm  --remove-all-storage
        fi
    fi
done