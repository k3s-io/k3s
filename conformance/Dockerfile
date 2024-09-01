FROM alpine:3.20
ENV SONOBUOY_VERSION 0.57.2
RUN apk add curl tar gzip
RUN curl -sfL https://github.com/vmware-tanzu/sonobuoy/releases/download/v${SONOBUOY_VERSION}/sonobuoy_${SONOBUOY_VERSION}_linux_amd64.tar.gz | tar xvzf - -C /usr/bin
COPY run-test.sh /usr/bin
CMD ["/usr/bin/run-test.sh"]
