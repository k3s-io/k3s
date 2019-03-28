FROM golang:1.12.1-alpine3.9

RUN apk -U --no-cache add bash git gcc musl-dev docker vim less file curl wget ca-certificates jq linux-headers zlib-dev tar zip squashfs-tools npm coreutils \
    python3 py3-pip python3-dev openssl-dev libffi-dev libseccomp libseccomp-dev make
RUN pip3 install 'tox==3.6.0'
RUN apk -U --no-cache --repository http://dl-3.alpinelinux.org/alpine/edge/main/ add sqlite-dev sqlite-static
RUN mkdir -p /go/src/golang.org/x && \
    cd /go/src/golang.org/x && git clone https://github.com/golang/tools && \
    git -C /go/src/golang.org/x/tools checkout -b current aa82965741a9fecd12b026fbb3d3c6ed3231b8f8 && \
    go install golang.org/x/tools/cmd/goimports
RUN rm -rf /go/src /go/pkg

ARG DAPPER_HOST_ARCH
ENV ARCH $DAPPER_HOST_ARCH

RUN if [ "${ARCH}" == "amd64" ]; then \
        curl -sL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s v1.15.0; \
    fi

ENV DAPPER_RUN_ARGS --privileged -v k3s-cache:/go/src/github.com/rancher/k3s/.cache
ENV DAPPER_ENV REPO TAG DRONE_TAG IMAGE_NAME
ENV DAPPER_SOURCE /go/src/github.com/rancher/k3s/
ENV DAPPER_OUTPUT ./bin ./dist ./build/out
ENV DAPPER_DOCKER_SOCKET true
ENV HOME ${DAPPER_SOURCE}
ENV CROSS true
ENV STATIC_BUILD true
WORKDIR ${DAPPER_SOURCE}

ENTRYPOINT ["./scripts/entry.sh"]
CMD ["ci"]

