FROM golang:1.11-alpine

RUN apk -U add bash git gcc musl-dev docker
RUN go get -d golang.org/x/lint/golint && \
    git -C /go/src/golang.org/x/lint/golint checkout -b current 06c8688daad7faa9da5a0c2f163a3d14aac986ca && \
    go install golang.org/x/lint/golint && \
    rm -rf /go/src /go/pkg

ENV DAPPER_SOURCE /go/src/github.com/rancher/norman/
ENV DAPPER_OUTPUT ./bin ./dist
ENV DAPPER_DOCKER_SOCKET true
ENV HOME ${DAPPER_SOURCE}
WORKDIR ${DAPPER_SOURCE}

ENTRYPOINT ["./scripts/entry"]
CMD ["ci"]
