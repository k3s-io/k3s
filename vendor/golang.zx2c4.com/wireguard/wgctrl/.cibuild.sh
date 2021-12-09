#!/usr/bin/env bash
set -e
set -x

# !! This script is meant for use in CI build use only !!

KERNEL=$(uname -s)

# Use doas in place of sudo for OpenBSD.
SUDO="sudo"
if [ "${KERNEL}" == "OpenBSD" ]; then
    SUDO="doas"
fi

if [ "${KERNEL}" == "Linux" ]; then
    # Configure a WireGuard interface.
    sudo ip link add wg0 type wireguard
    sudo ip link set up wg0
fi

# Set up wireguard-go on all OSes.
git clone git://git.zx2c4.com/wireguard-go
cd wireguard-go

if [ "${KERNEL}" == "Linux" ]; then
    # Bypass Linux compilation restriction.
    make
else
    # Build directly to avoid Makefile.
    go build -o wireguard-go
fi

${SUDO} mv ./wireguard-go /usr/local/bin/wireguard-go
cd ..
${SUDO} rm -rf ./wireguard-go
