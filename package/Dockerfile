FROM alpine:3.8 as base
RUN apk add -U ca-certificates
ADD build/out/data.tar.gz /image
RUN mkdir -p /image/etc/ssl/certs /image/run /image/var/run /image/tmp /image/lib/modules /image/lib/firmware && \
    cp /etc/ssl/certs/ca-certificates.crt /image/etc/ssl/certs/ca-certificates.crt
RUN cd image/bin && \
    rm -f k3s && \
    ln -s k3s-server k3s

FROM scratch
COPY --from=base /image /
VOLUME /var/lib/rancher/k3s 
VOLUME /var/lib/cni
VOLUME /var/log
ENTRYPOINT ["/bin/k3s"]
CMD ["agent"]
