ARG GO_VERSION=1.13
ARG BATS_VERSION=v1.2.0
ARG CRIU_VERSION=v3.14

FROM golang:${GO_VERSION}-buster
ARG DEBIAN_FRONTEND=noninteractive

RUN dpkg --add-architecture armel \
    && dpkg --add-architecture armhf \
    && dpkg --add-architecture arm64 \
    && dpkg --add-architecture ppc64el \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        crossbuild-essential-arm64 \
        crossbuild-essential-armel \
        crossbuild-essential-armhf \
        crossbuild-essential-ppc64el \
        curl \
        gawk \
        iptables \
        jq \
        kmod \
        libaio-dev \
        libcap-dev \
        libnet-dev \
        libnl-3-dev \
        libprotobuf-c-dev \
        libprotobuf-dev \
        libseccomp-dev \
        libseccomp-dev:arm64 \
        libseccomp-dev:armel \
        libseccomp-dev:armhf \
        libseccomp-dev:ppc64el \
        libseccomp2 \
        pkg-config \
        protobuf-c-compiler \
        protobuf-compiler \
        python-minimal \
        sudo \
        uidmap \
    && apt-get clean \
    && rm -rf /var/cache/apt /var/lib/apt/lists/*;

# Add a dummy user for the rootless integration tests. While runC does
# not require an entry in /etc/passwd to operate, one of the tests uses
# `git clone` -- and `git clone` does not allow you to clone a
# repository if the current uid does not have an entry in /etc/passwd.
RUN useradd -u1000 -m -d/home/rootless -s/bin/bash rootless

# install bats
ARG BATS_VERSION
RUN cd /tmp \
    && git clone https://github.com/bats-core/bats-core.git \
    && cd bats-core \
    && git reset --hard "${BATS_VERSION}" \
    && ./install.sh /usr/local \
    && rm -rf /tmp/bats-core

# install criu
ARG CRIU_VERSION
RUN mkdir -p /usr/src/criu \
    && curl -fsSL https://github.com/checkpoint-restore/criu/archive/${CRIU_VERSION}.tar.gz | tar -C /usr/src/criu/ -xz --strip-components=1 \
    && cd /usr/src/criu \
    && echo 1 > .gitid \
    && make -j $(nproc) install-criu \
    && cd - \
    && rm -rf /usr/src/criu

# install skopeo
RUN echo 'deb http://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/stable/Debian_Unstable/ /' > /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list \
    && wget -nv https://download.opensuse.org/repositories/devel:kubic:libcontainers:stable/Debian_Unstable/Release.key -O- | sudo apt-key add - \
    && apt-get update \
    && apt-get install -y --no-install-recommends skopeo \
    && rm -rf /etc/apt/sources.list.d/devel:kubic:libcontainers:stable.list \
    && apt-get clean \
    && rm -rf /var/cache/apt /var/lib/apt/lists/*;

# install umoci
RUN curl -o /usr/local/bin/umoci -fsSL https://github.com/opencontainers/umoci/releases/download/v0.4.5/umoci.amd64 \
    && chmod +x /usr/local/bin/umoci

COPY script/tmpmount /
WORKDIR /go/src/github.com/opencontainers/runc
ENTRYPOINT ["/tmpmount"]

# setup a playground for us to spawn containers in
COPY tests/integration/multi-arch.bash tests/integration/
ENV ROOTFS /busybox
RUN mkdir -p "${ROOTFS}"
RUN . tests/integration/multi-arch.bash \
    && curl -fsSL `get_busybox` | tar xfJC - "${ROOTFS}"

ENV DEBIAN_ROOTFS /debian
RUN mkdir -p "${DEBIAN_ROOTFS}"
RUN . tests/integration/multi-arch.bash \
    && get_and_extract_debian "$DEBIAN_ROOTFS"
