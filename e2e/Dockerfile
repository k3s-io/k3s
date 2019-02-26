FROM ubuntu:18.04
RUN apt-get update && \
    apt-get install -y curl
RUN curl -sfL https://github.com/heptio/sonobuoy/releases/download/v0.13.0/sonobuoy_0.13.0_linux_amd64.tar.gz | tar xvzf - -C /usr/bin
COPY run-test.sh /usr/bin
CMD ["/usr/bin/run-test.sh"]
