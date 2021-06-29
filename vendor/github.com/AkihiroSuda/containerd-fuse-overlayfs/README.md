# [`fuse-overlayfs`](https://github.com/containers/fuse-overlayfs) snapshotter plugin for [containerd](https://containerd.io)

Unlike `overlayfs`, `fuse-overlayfs` can be used as a non-root user without patching the kernel.

## Requirements
* kernel >= 4.18
* containerd >= 1.4
* fuse-overlayfs >= 0.7.0

## Setup

Two installation options are supported:
1. Embed `fuse-overlayfs` plugin into the containerd binary
2. Execute `fuse-overlayfs` plugin as a separate binary

Choose 1 if you don't mind recompiling containerd, otherwise choose 2.

### Option 1: Embed `fuse-overlayfs` plugin into the containerd binary

Create `builtins_fuseoverlayfs_linux.go` under [`$GOPATH/src/github.com/containerd/containerd/cmd/containerd`](https://github.com/containerd/containerd/tree/master/cmd/containerd)
with the following content, and recompile the containerd binary:

```go
/*
   Copyright The containerd Authors.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at
       http://www.apache.org/licenses/LICENSE-2.0
   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package main

// NOTE: the package name was "github.com/AkihiroSuda/containerd-fuse-overlayfs" before v1.0.0
import _ "github.com/AkihiroSuda/containerd-fuse-overlayfs/plugin"
```

No extra configuration is needed.

See https://github.com/containerd/containerd/blob/master/docs/rootless.md for how to run containerd as a non-root user.

### Option 2: Execute `fuse-overlayfs` plugin as a separate binary

* Install `containerd-fuse-overlayfs-grpc` binary. The binary will be installed under `$DESTDIR/bin`.
```console
$ make && DESTDIR=$HOME make install
```

* Create the following configuration in `~/.config/containerd/config.toml`:
```toml
version = 2
# substitute "/home/suda" with your own $HOME
root = "/home/suda/.local/share/containerd"
# substitute "/run/user/1001" with your own $XDG_RUNTIME_DIR
state = "/run/user/1001/containerd"

[grpc]
  address = "/run/user/1001/containerd/containerd.sock"

[proxy_plugins]
  [proxy_plugins."fuse-overlayfs"]
    type = "snapshot"
    address = "/run/user/1001/containerd/fuse-overlayfs.sock"
```

* Start [RootlessKit](https://github.com/rootless-containers/rootlesskit) with `sleep infinity` (or any kind of "pause" command):
```console
$ rootlesskit \
  --net=slirp4netns --disable-host-loopback \
  --copy-up=/etc --copy-up=/run \
  --state-dir=$XDG_RUNTIME_DIR/rootlesskit-containerd \
  sh -c "rm -rf /run/containerd ; sleep infinity"
```
(Note: `rm -rf /run/containerd` is a workaround for [containerd/containerd#2767](https://github.com/containerd/containerd/issues/2767))

* Enter the RootlessKit namespaces and run `containerd-fuse-overlayfs-grpc`:
```console
$ nsenter -U --preserve-credentials -m -n -t $(cat $XDG_RUNTIME_DIR/rootlesskit-containerd/child_pid) \
  containerd-fuse-overlayfs-grpc $XDG_RUNTIME_DIR/containerd/fuse-overlayfs.sock $HOME/.local/share/containerd-fuse-overlayfs
```

* Enter the same namespaces and run `containerd`:
```console
$ nsenter -U --preserve-credentials -m -n -t $(cat $XDG_RUNTIME_DIR/rootlesskit-containerd/child_pid) \
  containerd -c $HOME/.config/containerd/config.toml
```

## Usage

```console
$ export CONTAINERD_SNAPSHOTTER=fuse-overlayfs
$ ctr pull ...
$ ctr run ...
```

## How to test

To run the test as a non-root user, [RootlessKit](https://github.com/rootless-containers/rootlesskit) needs to be installed.

```console
$ go test -exec rootlesskit -test.v -test.root
```
