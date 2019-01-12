FROM ppc64le/alpine

LABEL maintainer="Tom Denham <tom@tigera.io>"

ENV FLANNEL_ARCH=ppc64le

ADD dist/qemu-$FLANNEL_ARCH-static /usr/bin/qemu-$FLANNEL_ARCH-static
RUN apk add --no-cache iproute2 net-tools ca-certificates iptables strongswan && update-ca-certificates
RUN apk add wireguard-tools --no-cache --repository http://dl-cdn.alpinelinux.org/alpine/edge/testing
COPY dist/flanneld-$FLANNEL_ARCH /opt/bin/flanneld
COPY dist/mk-docker-opts.sh /opt/bin/

ENTRYPOINT ["/opt/bin/flanneld"]

