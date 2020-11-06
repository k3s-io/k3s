FROM ubuntu:18.04
RUN apt-get update && \
    apt-get install -y curl
RUN curl -sfL https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.19.0/sonobuoy_0.19.0_linux_amd64.tar.gz | tar xvzf - -C /usr/bin
COPY run-test.sh /usr/bin
CMD ["/usr/bin/run-test.sh"]
