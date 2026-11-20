#!/usr/bin/env bash
# The *_SHA256_<arch> variables are read indirectly by sha()
# shellcheck disable=SC2034
# Installs the K3s developer tools on top of the K3s build environment
# (the "infra" stage of the repository's Dockerfile, which is Alpine based).
#
# Every download is pinned to a version and verified against a SHA256
# checksum. To bump a tool, change its version and both checksums below.
set -euo pipefail

HELM_VERSION=v4.3.2
HELM_SHA256_amd64=d1c0278d46beab79582d7e50f5b3dc206944ba8e577cde618513fd07daa53983
HELM_SHA256_arm64=df31cb61680ef728f82c770e5280dd565617bee6b039d37e85c677288be4bbb0

K9S_VERSION=v0.51.0
K9S_SHA256_amd64=c3752ad51a5a4015a113819c4eeb6e55a4d0e4b8e652494797532f6fc8161dd7
K9S_SHA256_arm64=3ee05c82e5f9198928a4e86133608ba6a2c10a2244d6a7789e820f78319d640c

# Keep in step with the version used by .github/workflows/validate.yaml
GOLANGCI_LINT_VERSION=v2.13.2
GOLANGCI_LINT_SHA256_amd64=2277d43b98ec0054280f2ac26b53268bae97682444678a59a657dd565da021d6
GOLANGCI_LINT_SHA256_arm64=a2a4e0065aa41be71f7c5ac90f271b61751331e5d04314e62afe4027855f0893

KUBERNETES_MCP_SERVER_VERSION=v0.1.3
KUBERNETES_MCP_SERVER_SHA256_amd64=363e43a7d865ad2d487f88a827b7d37cdd5b56b4ca6fd86affcd0e231a4104a6
KUBERNETES_MCP_SERVER_SHA256_arm64=262d3f3d7800b88574b6be30c1cb4663f81481fa12bd41e02c10d43a66069d3e

CLAUDE_CODE_VERSION=2.1.311
CLAUDE_CODE_SHA256_amd64=cd40dd327145ee47aa0deacd571801c1d782388f709d71242eee6b91de8ed11f
CLAUDE_CODE_SHA256_arm64=172150e3f9ea07e8e2e2e29862d7d9a25f5c234d8e909e4063df237665bf09b9

# Go tools are verified by the Go checksum database (sum.golang.org)
DELVE_VERSION=v1.27.2
GOPLS_VERSION=v0.23.0  # v0.24 needs Go 1.27; keep in step with the Go version in ../../../Dockerfile
GOTESTS_VERSION=v1.9.0

INSTALLCLAUDECODE=${INSTALLCLAUDECODE:-true}

case "$(uname -m)" in
  x86_64) ARCH=amd64 ;;
  aarch64) ARCH=arm64 ;;
  *) echo "k3s-dev: unsupported architecture $(uname -m), only amd64 and arm64 are supported" >&2; exit 1 ;;
esac

TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

# fetch URL SHA256 FILE: download URL to FILE and verify its checksum
fetch() {
  curl -fsSL --retry 3 -o "$3" "$1"
  echo "$2  $3" | sha256sum -c -
}

sha() { local v="$1_SHA256_${ARCH}"; echo "${!v}"; }

echo "k3s-dev: installing Alpine packages"
# The infra stage already provides bash, git, docker, curl, jq, yq, make, gcc and the static libraries
apk add --no-cache \
  bash-completion \
  docker-cli-buildx \
  docker-bash-completion \
  figlet \
  github-cli \
  iproute2 \
  iptables \
  ip6tables \
  kmod \
  libgcc \
  libstdc++ \
  procps \
  ripgrep \
  shellcheck

echo "k3s-dev: installing helm ${HELM_VERSION}"
fetch "https://get.helm.sh/helm-${HELM_VERSION}-linux-${ARCH}.tar.gz" "$(sha HELM)" "${TMP}/helm.tgz"
tar -xzf "${TMP}/helm.tgz" -C "${TMP}" "linux-${ARCH}/helm"
install -m 0755 "${TMP}/linux-${ARCH}/helm" /usr/local/bin/helm

echo "k3s-dev: installing k9s ${K9S_VERSION}"
fetch "https://github.com/derailed/k9s/releases/download/${K9S_VERSION}/k9s_Linux_${ARCH}.tar.gz" "$(sha K9S)" "${TMP}/k9s.tgz"
tar -xzf "${TMP}/k9s.tgz" -C "${TMP}" k9s
install -m 0755 "${TMP}/k9s" /usr/local/bin/k9s

echo "k3s-dev: installing golangci-lint ${GOLANGCI_LINT_VERSION}"
GL="golangci-lint-${GOLANGCI_LINT_VERSION#v}-linux-${ARCH}"
fetch "https://github.com/golangci/golangci-lint/releases/download/${GOLANGCI_LINT_VERSION}/${GL}.tar.gz" "$(sha GOLANGCI_LINT)" "${TMP}/golangci-lint.tgz"
tar -xzf "${TMP}/golangci-lint.tgz" -C "${TMP}" "${GL}/golangci-lint"
install -m 0755 "${TMP}/${GL}/golangci-lint" /usr/local/bin/golangci-lint

echo "k3s-dev: installing kubernetes-mcp-server ${KUBERNETES_MCP_SERVER_VERSION}"
fetch "https://github.com/containers/kubernetes-mcp-server/releases/download/${KUBERNETES_MCP_SERVER_VERSION}/kubernetes-mcp-server-linux-${ARCH}" \
  "$(sha KUBERNETES_MCP_SERVER)" "${TMP}/kubernetes-mcp-server"
install -m 0755 "${TMP}/kubernetes-mcp-server" /usr/local/bin/kubernetes-mcp-server

if [ "${INSTALLCLAUDECODE}" = "true" ]; then
  echo "k3s-dev: installing Claude Code ${CLAUDE_CODE_VERSION}"
  CC_ARCH=$([ "${ARCH}" = amd64 ] && echo x64 || echo arm64)
  fetch "https://downloads.claude.ai/claude-code-releases/${CLAUDE_CODE_VERSION}/linux-${CC_ARCH}-musl/claude" \
    "$(sha CLAUDE_CODE)" "${TMP}/claude"
  install -m 0755 "${TMP}/claude" /usr/local/bin/claude
fi

echo "k3s-dev: installing delve ${DELVE_VERSION}, gopls ${GOPLS_VERSION}, gotests ${GOTESTS_VERSION}"
# The infra stage builds statically for cross compilation; Go tools are plain native builds
export GOBIN=/usr/local/bin CGO_ENABLED=0 GOFLAGS=-modcacherw
unset GOOS GOARCH
go install "github.com/go-delve/delve/cmd/dlv@${DELVE_VERSION}"
go install "golang.org/x/tools/gopls@${GOPLS_VERSION}"
go install "github.com/cweill/gotests/gotests@${GOTESTS_VERSION}"
go clean -cache -modcache

echo "k3s-dev: installing the container init script and shell integration"
FEATURE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
install -D -m 0755 "${FEATURE_DIR}/init.sh" /usr/local/share/k3s-dev/init.sh
install -D -m 0644 "${FEATURE_DIR}/daemon.json" /etc/docker/daemon.json
install -D -m 0644 "${FEATURE_DIR}/profile.sh" /etc/profile.d/k3s-dev.sh

# Use bash as root's login shell (Alpine defaults to ash)
sed -i 's|^root:\(.*\):/bin/ash$|root:\1:/bin/bash|' /etc/passwd

# Interactive shells in VS Code are non-login bash shells, so load the profile from bashrc too
touch /root/.bashrc
grep -q 'k3s-dev.sh' /root/.bashrc || echo '. /etc/profile.d/k3s-dev.sh' >> /root/.bashrc
echo '[ -f ~/.bashrc ] && . ~/.bashrc' > /root/.bash_profile

echo "k3s-dev: done"
