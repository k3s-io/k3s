### BASE ###
FROM alpine:3.8 as base
RUN apk -U add findutils iptables ipset bash ca-certificates jq iproute2 nfs-utils coreutils libseccomp conntrack-tools
# RUN apk -U add eudev tinyssh e2fsprogs mdadm rsync nfs-utils parted

FROM base as k3s-build
COPY --from=base /bin /usr/src/image/bin/
COPY --from=base /lib /usr/src/image/lib/
COPY --from=base /sbin /usr/src/image/sbin/
COPY --from=base /etc/ssl/certs/ca-certificates.crt /usr/src/image/etc/ssl/certs/ca-certificates.crt
COPY --from=base /etc/terminfo /usr/src/image/etc/terminfo
COPY --from=base /usr /usr/src/image/usr/

WORKDIR /usr/src/image

RUN rm -rf  usr/bin/iconv \
            usr/bin/scanelf \
            usr/bin/ssl_client \
            usr/bin/pkgconf \
            usr/bin/getent \
            usr/bin/locate \
            usr/bin/updatedb \
            usr/bin/c_rehash \
            usr/bin/getconf \
            usr/etc \
            usr/include \
            usr/lib/bash \
            usr/lib/krb5 \
            usr/lib/pkgconfig \
            usr/lib/tc \
            usr/libexec \
            usr/local \
            usr/sbin/nfsiostat \
            usr/sbin/rpc.gssd \
            usr/sbin/nfsidmap \
            usr/sbin/blkmapd \
            usr/sbin/conntrackd \
            usr/sbin/nfct \
            usr/sbin/nfsstat \
            usr/sbin/mountstats \
            usr/sbin/setcap \
            usr/sbin/exportfs \
            usr/sbin/update-ca-certificates \
            usr/sbin/capsh \
            usr/sbin/getcap \
            usr/sbin/rpcdebug \
            usr/sbin/start-statd \
            usr/sbin/getpcaps \
            usr/sbin/sm-notify \
            usr/share/aclocal \
            usr/share/apk \
            usr/share/ca-certificates \
            usr/share/man \
            usr/share/misc && \
            find usr/share/terminfo -type f -exec rm {} \; && \
            ln -s xterm-color usr/share/terminfo/x/xterm-256color && \
            rmdir usr/share/terminfo/* || true
RUN rm -rf  bin/sh \
            lib/apk \
            lib/mdev \
            sbin/ss \
            sbin/routel \
            sbin/*-compat* \
            sbin/genl \
            sbin/lnstat \
            sbin/ifstat \
            sbin/mkmntdirs \
            sbin/nfsdcltrack \
            sbin/rtacct \
            sbin/nstat \
            sbin/routef \
            sbin/apk \
            sbin/tc \
            sbin/ifcfg \
            sbin/setup-udev \
            sbin/rtpr \
            sbin/osd_login \
            sbin/bridge \
            sbin/rtmon && \
    ln -s bash bin/sh && \
    mkdir -p lib/modules

RUN mv sbin/* bin/ && \
    rmdir sbin && \
    ln -s bin sbin

RUN mkdir lib2 && \
    mv usr/lib/* lib2/ && \
    mv lib2/* lib/ && \
    mv usr/bin/* bin/ && \
    mv usr/sbin/* bin/ && \
    mv usr/share . && \
    rm -rf usr lib2 && \
    for i in $(ls -l bin | grep usr/bin/coreutils | awk '{print $(NF-2)}'); do \
        rm bin/$i && ln -s coreutils bin/$i \
    ;done && \
    find -L bin -type l -exec rm {} \; -print

RUN apk add upx && \
    upx $(find bin -type f -executable \! -name coreutils) || true

RUN echo '#### LAYOUT #####' && \
    find -type d && \
    echo '#### BIN #####' && \
    find bin -type f -executable && \
    du -x -s -h

RUN tar cf ../rootfs.tar * && \
    ls -la ../rootfs.tar

CMD ["sh"]

### BUILD IMAGE ###
FROM golang:1.11-alpine AS gobuild
RUN apk -U add git gcc linux-headers musl-dev make libseccomp libseccomp-dev bash
RUN rm -f /bin/sh && ln -s /bin/bash /bin/sh

### CNI ###
FROM gobuild AS cni
RUN mkdir -p $GOPATH/src/github.com/containernetworking && \
    cd $GOPATH/src/github.com/containernetworking && \
    git clone https://github.com/ibuildthecloud/plugins.git && \
    cd plugins && \
    git checkout 9810b7d5137b171c4e07ce59bb18be9feccec557
RUN go build -buildmode=pie -ldflags -s -o /usr/bin/cni github.com/containernetworking/plugins

### RUNC ###
FROM gobuild AS runc
RUN go get -d github.com/opencontainers/runc && \
    git -C $GOPATH/src/github.com/opencontainers/runc checkout -b build v1.0.0-rc5
WORKDIR $GOPATH/src/github.com/opencontainers/runc
RUN make runc && \
    cp runc /usr/bin/

### CONTAINERD ###
FROM gobuild AS containerd
RUN go get -d github.com/containerd/containerd && \
    git -C $GOPATH/src/github.com/containerd/containerd checkout -b build v1.1.4
WORKDIR $GOPATH/src/github.com/containerd/containerd
RUN sed -i -e '/aufs/d' -e '/zfs/d' cmd/containerd/builtins_linux.go
RUN make BUILDTAGS="apparmor seccomp no_btrfs netgo osusergo" bin/containerd bin/containerd-shim && \
    cp bin/containerd bin/containerd-shim /usr/bin/

### AGENT ###
FROM gobuild AS agent
ADD /build/vendor.tar $GOPATH/src/github.com/rancher/rio/
COPY /agent $GOPATH/src/github.com/rancher/rio/agent
WORKDIR $GOPATH/src/github.com/rancher/rio
RUN go build -buildmode=pie -tags k3s -ldflags -s -o /usr/bin/agent ./agent

### ASSEMBLE IMAGE ###
FROM gobuild

RUN apk add -U squashfs-tools

COPY --from=k3s-build /usr/src/rootfs.tar /usr/src/rootfs.tar
RUN mkdir /usr/src/image && \
    tar xf /usr/src/rootfs.tar -C /usr/src/image

COPY --from=runc /usr/bin/runc /usr/src/image/bin/runc
RUN strip --strip-unneeded /usr/src/image/bin/runc

COPY --from=agent /usr/bin/agent /usr/src/image/bin/
RUN strip --strip-unneeded /usr/src/image/bin/agent

COPY --from=containerd /usr/bin/containerd-shim /usr/bin/containerd /usr/src/image/bin/
#COPY containerd /usr/src/image/bin
RUN strip --strip-unneeded /usr/src/image/bin/containerd /usr/src/image/bin/containerd-shim

RUN mkdir -p /usr/src/image/share/cni/bin
COPY --from=cni /usr/bin/cni /usr/src/image/share/cni/bin
RUN cd /usr/src/image/share/cni/bin && \
    for i in ./bridge ./flannel ./host-local ./loopback ./portmap; do \
        ln -s cni $i \
    ;done

COPY image/init /usr/src/image/init

RUN mksquashfs /usr/src/image main.squashfs
