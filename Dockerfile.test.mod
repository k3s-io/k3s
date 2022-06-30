ARG GOLANG=golang:1.18.1-alpine3.15
FROM ${GOLANG}

RUN apk -U --no-cache add bash jq
ENV K3S_SOURCE /go/src/github.com/k3s-io/k3s/
WORKDIR ${K3S_SOURCE}

COPY ./scripts/test-mods /bin/
COPY . ${K3S_SOURCE}

ENTRYPOINT ["/bin/test-mods"]