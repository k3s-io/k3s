FROM golang:1.9.2-alpine3.6

ENV GOPATH /go
ENV USER root

COPY . /go/src/github.com/cloudflare/cfssl

RUN set -x && \
	apk --no-cache add git gcc libc-dev && \
	go get github.com/cloudflare/cfssl_trust/... && \
	go get github.com/GeertJohan/go.rice/rice && \
	cd /go/src/github.com/cloudflare/cfssl && rice embed-go -i=./cli/serve && \
	mkdir bin && cd bin && \
	go build ../cmd/cfssl && \
	go build ../cmd/cfssljson && \
	go build ../cmd/mkbundle && \
	go build ../cmd/multirootca && \
	echo "Build complete."

FROM alpine:3.6
COPY --from=0 /go/src/github.com/cloudflare/cfssl_trust /etc/cfssl
COPY --from=0 /go/src/github.com/cloudflare/cfssl/bin/ /usr/bin

VOLUME [ "/etc/cfssl" ]
WORKDIR /etc/cfssl

EXPOSE 8888

ENTRYPOINT ["cfssl"]
CMD ["--help"]
