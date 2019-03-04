FROM golang:1.10-stretch

RUN dpkg --add-architecture armel \
    && dpkg --add-architecture armhf \
    && dpkg --add-architecture arm64 \
    && dpkg --add-architecture ppc64el \
    && apt-get update && apt-get install -y \
    build-essential \
    curl \
    sudo \
    gawk \
    iptables \
    jq \
    pkg-config \
    libaio-dev \
    libcap-dev \
    libprotobuf-dev \
    libprotobuf-c0-dev \
    libnl-3-dev \
    libnet-dev \
    libseccomp2 \
    libseccomp-dev \
    protobuf-c-compiler \
    protobuf-compiler \
    python-minimal \
    uidmap \
    kmod \
    crossbuild-essential-armel crossbuild-essential-armhf crossbuild-essential-arm64 crossbuild-essential-ppc64el \
    libseccomp-dev:armel libseccomp-dev:armhf libseccomp-dev:arm64 libseccomp-dev:ppc64el \
    --no-install-recommends \
    && apt-get clean

# Add a dummy user for the rootless integration tests. While runC does
# not require an entry in /etc/passwd to operate, one of the tests uses
# `git clone` -- and `git clone` does not allow you to clone a
# repository if the current uid does not have an entry in /etc/passwd.
RUN useradd -u1000 -m -d/home/rootless -s/bin/bash rootless

# install bats
RUN cd /tmp \
    && git clone https://github.com/sstephenson/bats.git \
    && cd bats \
    && git reset --hard 03608115df2071fff4eaaff1605768c275e5f81f \
    && ./install.sh /usr/local \
    && rm -rf /tmp/bats

# install criu
# For CRIU 3.11 one patch is needed for the test 'checkpoint and restore with container specific CRIU config'
# This should be no longer necessary with CRIU 3.12
# See https://github.com/opencontainers/runc/pull/1933
ENV CRIU_VERSION v3.11
RUN mkdir -p /usr/src/criu \
    && curl -sSL https://github.com/checkpoint-restore/criu/archive/${CRIU_VERSION}.tar.gz | tar -v -C /usr/src/criu/ -xz --strip-components=1 \
    && cd /usr/src/criu \
    && curl https://github.com/checkpoint-restore/criu/commit/bb0b2f2635d71e549851b7c626a1464e42a3b5c7.patch | patch -p1 \
    && make install-criu \
    && rm -rf /usr/src/criu

# setup a playground for us to spawn containers in
ENV ROOTFS /busybox
RUN mkdir -p ${ROOTFS}

COPY script/tmpmount /
WORKDIR /go/src/github.com/opencontainers/runc
ENTRYPOINT ["/tmpmount"]

ADD . /go/src/github.com/opencontainers/runc

RUN . tests/integration/multi-arch.bash \
    && curl -o- -sSL `get_busybox` | tar xfJC - ${ROOTFS}
