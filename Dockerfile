ARG GOLANG=golang:1.26.4-alpine3.22
ARG XX=tonistiigi/xx:1.6.1

FROM --platform=$BUILDPLATFORM ${XX} AS xx

FROM --platform=$BUILDPLATFORM ${GOLANG} AS infra
COPY --from=xx / /
ARG TARGETOS
ARG TARGETARCH
ENV TARGETOS=$TARGETOS
ENV TARGETARCH=$TARGETARCH

RUN apk -U --no-cache add bash git docker vim less file curl wget ca-certificates jq \
    tar zip squashfs-tools coreutils openssl-dev libffi-dev make libuv-static \
    zstd pigz alpine-sdk yq clang lld \
    && \
    if [ "${TARGETARCH}" = "arm64" ]; then \
    apk -U --no-cache add binutils-gold; \
    fi \
    && \
    if [ "${TARGETOS}" = "windows" ] && [ "${TARGETARCH}" = "amd64" ]; then \
    apk -U --no-cache add mingw-w64-gcc; \
    fi

RUN if [ "${TARGETOS}" = "linux" ]; then \
      xx-apk add --no-cache gcc musl-dev linux-headers zlib-dev zlib-static libseccomp libseccomp-dev libseccomp-static sqlite-dev sqlite-static libselinux libselinux-dev btrfs-progs-dev btrfs-progs-static; \
    fi

# Install goimports
RUN GOPROXY=direct go install golang.org/x/tools/cmd/goimports@gopls/v0.20.0
RUN rm -rf /go/src /go/pkg

ARG SELINUX=true
ENV SELINUX=$SELINUX
ENV STATIC_BUILD=true
ENV SRC_DIR=/go/src/github.com/k3s-io/k3s
WORKDIR ${SRC_DIR}/


FROM infra AS manifests
ARG GIT_TAG
ARG TREE_STATE
ARG COMMIT
ARG DIRTY
ARG GOOS
ARG TARGETOS
ARG TARGETARCH
ENV TARGETOS=$TARGETOS
ENV TARGETARCH=$TARGETARCH
# Used by both build and validate stages, better caching if we do this in a separate stage
COPY ./scripts/ ./scripts
COPY ./go.mod ./go.sum ./main.go ./
COPY ./manifests ./manifests
RUN mkdir -p bin dist
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod \
    ./scripts/download


FROM manifests AS validate
ARG SKIP_VALIDATE
COPY . .
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod \
    --mount=type=cache,id=gobuild,target=/root/.cache/go-build \
    --mount=type=cache,id=lint,target=/root/.cache/golangci-lint \
    ./scripts/validate


FROM manifests AS build
ARG GOCOVER
ARG DEBUG
COPY ./cmd ./cmd
COPY ./tests ./tests
COPY ./pkg ./pkg
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod \
    --mount=type=cache,id=gobuild-${TARGETOS}-${TARGETARCH},target=/root/.cache/go-build \
    if [ "${TARGETOS}" = "linux" ]; then GO=xx-go ./scripts/build; else ./scripts/build; fi

COPY ./contrib ./contrib
RUN --mount=type=cache,id=gomod,target=/go/pkg/mod \
    --mount=type=cache,id=gobuild-${TARGETOS}-${TARGETARCH},target=/root/.cache/go-build \
    if [ "${TARGETOS}" = "linux" ]; then GO=xx-go ./scripts/package-cli; else ./scripts/package-cli; fi

RUN ./scripts/binary_size_check.sh

FROM scratch AS result
ENV SRC_DIR=/go/src/github.com/k3s-io/k3s
COPY --from=build ${SRC_DIR}/dist /dist
COPY --from=build ${SRC_DIR}/bin /bin
COPY --from=build ${SRC_DIR}/build/out /build/out
COPY --from=build ${SRC_DIR}/build/static /build/static
COPY --from=build ${SRC_DIR}/pkg/static /pkg/static
COPY --from=build ${SRC_DIR}/pkg/deploy /pkg/deploy

# For publishing artifacts, we only need mutlicall binaries and data tarballs.
FROM scratch AS multiarch-result
ENV SRC_DIR=/go/src/github.com/k3s-io/k3s
COPY --from=build ${SRC_DIR}/dist /dist
COPY --from=build ${SRC_DIR}/build/out /build/out