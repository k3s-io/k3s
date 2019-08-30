# This dockerfile is used to test containerd within a container
#
# usage:
# 1.) docker build -t containerd-test -f Dockerfile.test ../
# 2.) docker run -it --privileged -v /tmp:/tmp --tmpfs /var/lib/containerd-test containerd-test  bash
# 3.) $ make binaries install test
#

ARG GOLANG_VERSION=1.12

FROM golang:${GOLANG_VERSION} AS golang-base
RUN mkdir -p /go/src/github.com/containerd/containerd
WORKDIR /go/src/github.com/containerd/containerd

# Install proto3
FROM golang-base AS proto3
RUN apt-get update && apt-get install -y \
   autoconf \
   automake \
   g++ \
   libtool \
   unzip \
 --no-install-recommends

COPY script/setup/install-protobuf install-protobuf
RUN ./install-protobuf

# Install runc
FROM golang-base AS runc
RUN apt-get update && apt-get install -y \
    curl \
    libseccomp-dev \
  --no-install-recommends

COPY vendor.conf vendor.conf
COPY script/setup/install-runc install-runc
RUN ./install-runc

FROM golang-base AS dev
RUN apt-get update && apt-get install -y \
    btrfs-tools \
    gcc \
    git \
    libseccomp-dev \
    make \
    xfsprogs \
  --no-install-recommends

COPY --from=proto3 /usr/local/bin/protoc     /usr/local/bin/protoc
COPY --from=proto3 /usr/local/include/google /usr/local/include/google
COPY --from=runc   /usr/local/sbin/runc      /usr/local/go/bin/runc

COPY . .
