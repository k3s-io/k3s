FROM arm64v8/alpine

LABEL maintainer="Tom Denham <tom@tigera.io>"

ENV FLANNEL_ARCH=arm64

ADD dist/qemu-aarch64-static /usr/bin/qemu-aarch64-static
RUN apk add --no-cache iproute2 net-tools ca-certificates iptables strongswan && update-ca-certificates
RUN apk add wireguard-tools --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing
COPY dist/flanneld-$FLANNEL_ARCH /opt/bin/flanneld
COPY dist/mk-docker-opts.sh /opt/bin/

ENTRYPOINT ["/opt/bin/flanneld"]

