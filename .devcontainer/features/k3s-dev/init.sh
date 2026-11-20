#!/usr/bin/env bash
# Entrypoint for the K3s dev container. Prepares the container so that K3s and
# Docker can run nested inside it, then hands over to the container command.
#
# K3s does the cgroup v2 part itself when it runs as PID 1 (see
# EvacuateCgroup2 in pkg/cli/cmds/init_linux.go, used by the rancher/k3s
# image). In a dev container K3s is not PID 1, so it is done here instead.

log() { echo "[k3s-dev init] $*"; }

# cgroup v2: move every process out of the container's root cgroup so
# controllers can be delegated to the kubelet and dockerd (same approach as
# moby's hack/dind script).
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
  mkdir -p /sys/fs/cgroup/init
  for _ in 1 2 3 4 5; do
    xargs -rn1 < /sys/fs/cgroup/cgroup.procs > /sys/fs/cgroup/init/cgroup.procs 2>/dev/null || :
    if sed -e 's/ / +/g' -e 's/^/+/' < /sys/fs/cgroup/cgroup.controllers \
        > /sys/fs/cgroup/cgroup.subtree_control 2>/dev/null; then
      break
    fi
    sleep 1
  done
fi

# Pod volume mounts need shared mount propagation
mount --make-rshared / 2>/dev/null || log "warning: could not make / rshared, is the container privileged?"

# Docker daemon for `make`, the tests/docker suite and extra agent nodes
if [ "${K3S_DEV_DOCKER:-true}" = "true" ] && ! pgrep -x dockerd > /dev/null; then
  log "starting dockerd (log: /var/log/dockerd.log)"
  rm -f /var/run/docker.pid
  nohup dockerd > /var/log/dockerd.log 2>&1 &
fi

exec "$@"
