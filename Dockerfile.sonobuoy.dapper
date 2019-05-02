FROM golang:1.12.1-alpine3.9

RUN apk -U --no-cache add bash git docker curl jq coreutils
RUN go get -d github.com/heptio/sonobuoy && \
    git -C /go/src/github.com/heptio/sonobuoy checkout -b current v0.14.2 && \
    go install github.com/heptio/sonobuoy
RUN rm -rf /go/src /go/pkg

ARG DAPPER_HOST_ARCH
ENV ARCH $DAPPER_HOST_ARCH

RUN curl -sL https://storage.googleapis.com/kubernetes-release/release/$( \
            curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt \
        )/bin/linux/${ARCH}/kubectl -o /usr/local/bin/kubectl && \
    chmod a+x /usr/local/bin/kubectl

ENV DAPPER_RUN_ARGS --privileged --network host
ENV DAPPER_ENV REPO TAG DRONE_TAG IMAGE_NAME
ENV DAPPER_SOURCE /go/src/github.com/rancher/k3s/
ENV DAPPER_OUTPUT ./dist
ENV DAPPER_DOCKER_SOCKET true
ENV HOME ${DAPPER_SOURCE}
WORKDIR ${DAPPER_SOURCE}

ENTRYPOINT ["./scripts/entry.sh"]
CMD ["sonobuoy-e2e-tests"]
