#/bin/bash

# Cross-compile for RISC-V in an Ubuntu 22.04 container

# To run mount a clean checkout of the code inside the container and run the script:
#
#   docker run --rm --mount type=bind,source=${PWD},target=/k3s ubuntu:22.04 /bin/bash -c /k3s/cross-compile-riscv.sh

dpkg --add-architecture riscv64
apt-get update
apt-get install -y wget curl git gcc-riscv64-linux-gnu g++-riscv64-linux-gnu pkg-config libseccomp-dev:riscv64 make zstd

HOST_ARCH=$(uname -m)
if [ ${HOST_ARCH} = aarch64 ]; then
    HOST_ARCH=arm64
fi

wget -P /tmp "https://dl.google.com/go/go1.20.5.linux-${HOST_ARCH}.tar.gz"
(cd /; tar -C /usr/local -xzf "/tmp/go1.20.5.linux-${HOST_ARCH}.tar.gz")
rm -rf /tmp/go1.20.5.linux-${HOST_ARCH}.tar.gz
mkdir -p /go/src /go/bin

export GOPATH=/go
export PATH=$GOPATH/bin:/usr/local/go/bin:$PATH

wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/${HOST_ARCH}
chmod +x /usr/local/bin/yq

cd /k3s
export ARCH=riscv64
bash -x ./scripts/download
bash -x ./scripts/build
bash -x ./scripts/package-cli
