FROM alpine:3.9 as base
RUN apk add -U ca-certificates
ADD build/out/data.tar.gz /image
RUN mkdir -p /image/etc/ssl/certs /image/run /image/var/run /image/tmp /image/lib/modules /image/lib/firmware && \
    cp /etc/ssl/certs/ca-certificates.crt /image/etc/ssl/certs/ca-certificates.crt
RUN cd image/bin && \
    rm -f k3s && \
    ln -s k3s-server k3s

FROM scratch
COPY --from=base /image /
RUN chmod 1777 /tmp
VOLUME /var/lib/rancher/k3s 
VOLUME /var/lib/cni
VOLUME /var/log
ENV PATH="$PATH:/bin/aux"
ENTRYPOINT ["/bin/k3s"]
CMD ["agent"]
