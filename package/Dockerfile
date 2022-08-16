FROM alpine:3.15 as base
RUN apk add -U ca-certificates tar zstd
COPY build/out/data.tar.zst /
RUN mkdir -p /image/etc/ssl/certs /image/run /image/var/run /image/tmp /image/lib/modules /image/lib/firmware && \
    tar -xa -C /image -f /data.tar.zst && \
    cp /etc/ssl/certs/ca-certificates.crt /image/etc/ssl/certs/ca-certificates.crt

ADD ./package/entrypoint.sh /bin

FROM scratch
ARG VERSION="dev"
ARG PROG="k3s"

COPY --from=base /image /
RUN mkdir -p /etc && \
    echo 'hosts: files dns' > /etc/nsswitch.conf && \
    echo "PRETTY_NAME=\"${PROG} ${VERSION}\"" > /etc/os-release && \
    chmod 1777 /tmp
VOLUME /var/lib/kubelet
VOLUME /var/lib/rancher/${PROG}
VOLUME /var/lib/cni
VOLUME /var/log
ENV PATH="$PATH:/bin/aux"
ENV CRI_CONFIG_FILE="/var/lib/rancher/${PROG}/agent/etc/crictl.yaml"
ENV PROG=${PROG}

ENTRYPOINT ["entrypoint.sh"]
CMD ["agent"]
