#!/bin/bash

echo "vm.panic_on_oom=0
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
kernel.keys.root_maxbytes=25000000
" >> /etc/sysctl.d/90-kubelet.conf
sysctl -p /etc/sysctl.d/90-kubelet.conf