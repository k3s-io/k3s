ARG FUSEOVERLAYFS_COMMIT=v1.2.0
ARG ROOTLESSKIT_COMMIT=v0.11.0
ARG SHADOW_COMMIT=4.8.1

FROM golang:1.15-alpine AS containerd-fuse-overlayfs-test
COPY . /go/src/github.com/AkihiroSuda/containerd-fuse-overlayfs
WORKDIR  /go/src/github.com/AkihiroSuda/containerd-fuse-overlayfs
ENV CGO_ENABLED=0
ENV GO111MODULE=off
RUN mkdir /out && go test -c -o /out/containerd-fuse-overlayfs.test

# from https://github.com/containers/fuse-overlayfs/blob/53c17dab78b43de1cd121bf9260b20b76371bbaf/Dockerfile.static.ubuntu
FROM docker.io/debian:10 AS fuse-overlayfs
RUN apt-get update && \
    apt-get install --no-install-recommends -y \
        git ca-certificates libc6-dev gcc g++ make automake autoconf clang pkgconf libfuse3-dev
ARG FUSEOVERLAYFS_REPO
RUN git clone https://github.com/containers/fuse-overlayfs
WORKDIR fuse-overlayfs
ARG FUSEOVERLAYFS_COMMIT
RUN git pull && git checkout ${FUSEOVERLAYFS_COMMIT}
RUN  ./autogen.sh && \
     LIBS="-ldl" LDFLAGS="-static" ./configure && \
     make && mkdir /out && cp fuse-overlayfs /out

FROM golang:1.15-alpine AS rootlesskit
RUN apk add --no-cache git
RUN git clone https://github.com/rootless-containers/rootlesskit.git /go/src/github.com/rootless-containers/rootlesskit
WORKDIR /go/src/github.com/rootless-containers/rootlesskit
ARG ROOTLESSKIT_COMMIT
RUN git pull && git checkout ${ROOTLESSKIT_COMMIT}
ENV CGO_ENABLED=0
RUN mkdir /out && go build -o /out/rootlesskit github.com/rootless-containers/rootlesskit/cmd/rootlesskit 

FROM alpine:3.12 AS idmap
RUN apk add --no-cache autoconf automake build-base byacc gettext gettext-dev gcc git libcap-dev libtool libxslt
RUN git clone https://github.com/shadow-maint/shadow.git
WORKDIR shadow
ARG SHADOW_COMMIT
RUN git pull && git checkout ${SHADOW_COMMIT}
RUN ./autogen.sh --disable-nls --disable-man --without-audit --without-selinux --without-acl --without-attr --without-tcb --without-nscd && \
   make && mkdir -p /out && cp src/newuidmap src/newgidmap /out

FROM alpine:3.12
COPY --from=containerd-fuse-overlayfs-test /out/containerd-fuse-overlayfs.test /usr/local/bin
COPY --from=rootlesskit /out/rootlesskit /usr/local/bin
COPY --from=fuse-overlayfs /out/fuse-overlayfs /usr/local/bin
COPY --from=idmap /out/newuidmap /usr/bin/newuidmap
COPY --from=idmap /out/newgidmap /usr/bin/newgidmap
RUN apk add --no-cache fuse3 libcap && \
    setcap CAP_SETUID=ep /usr/bin/newuidmap && \
    setcap CAP_SETGID=ep /usr/bin/newgidmap && \
    adduser -D -u 1000 testuser && \
    echo testuser:100000:65536 | tee /etc/subuid | tee /etc/subgid
USER testuser
# If /tmp is real overlayfs, some tests fail. Mount a volume to ensure /tmp to be a sane filesystem.
VOLUME /tmp
# requires --security-opt seccomp=unconfined --security-opt apparmor=unconfined --device /dev/fuse 
CMD ["rootlesskit", "containerd-fuse-overlayfs.test", "-test.root", "-test.v"]
