# [`fuse-overlayfs`](https://github.com/containers/fuse-overlayfs) snapshotter plugin for [containerd](https://containerd.io)

Unlike `overlayfs`, `fuse-overlayfs` can be used as a non-root user on almost all recent distros.

You do NOT need this `fuse-overlayfs` plugin on the following environments, because they support the real `overlayfs` for non-root users:
- [kernel >= 5.11](https://github.com/torvalds/linux/commit/459c7c565ac36ba09ffbf24231147f408fde4203)
- [Ubuntu kernel, since circa 2015](https://kernel.ubuntu.com/git/ubuntu/ubuntu-bionic.git/commit/fs/overlayfs?id=3b7da90f28fe1ed4b79ef2d994c81efbc58f1144)
- [Debian 10 kernel](https://salsa.debian.org/kernel-team/linux/blob/283390e7feb21b47779b48e0c8eb0cc409d2c815/debian/patches/debian/overlayfs-permit-mounts-in-userns.patch)
  - Debian 10 needs `sudo modprobe overlay permit_mounts_in_userns=1`. Future release of Debian with kernel >= 5.11 will not need this `modprobe` hack.

fuse-overlayfs-snapshotter is a **non-core** sub-project of containerd.

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

import _ "github.com/containerd/fuse-overlayfs-snapshotter/plugin"
```

No extra configuration is needed.

See https://github.com/containerd/containerd/blob/master/docs/rootless.md for how to run containerd as a non-root user.

### Option 2: Execute `fuse-overlayfs` plugin as a separate binary

#### "Easy way"

The easiest way is to use `containerd-rootless-setuptool.sh` included in [nerdctl](https://github.com/containerd/nerdctl).

```console
$ containerd-rootless-setuptool.sh install
$ containerd-rootless-setuptool.sh install-fuse-overlayfs
[INFO] Creating "/home/exampleuser/.config/systemd/user/containerd-fuse-overlayfs.service"
...
[INFO] Installed "containerd-fuse-overlayfs.service" successfully.
[INFO] To control "containerd-fuse-overlayfs.service", run: `systemctl --user (start|stop|restart) containerd-fuse-overlayfs.service`
[INFO] Add the following lines to "/home/exampleuser/.config/containerd/config.toml" manually:
### BEGIN ###
[proxy_plugins]
  [proxy_plugins."fuse-overlayfs"]
    type = "snapshot"
    address = "/run/user/1000/containerd-fuse-overlayfs.sock"
###  END  ###
[INFO] Set `export CONTAINERD_SNAPSHOTTER="fuse-overlayfs"` to use the fuse-overlayfs snapshotter.
```

Add the `[proxy_plugins."fuse-overlayfs"]` configuration shown above to `~/.config/containerd/config.toml`.
"1000" needs to be replaced with your actual UID.

#### "Hard way"

<details>
<summary>Click here to show the "hard way"</summary>

<p>

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

</p>
</details>

## Usage

```console
$ export CONTAINERD_SNAPSHOTTER=fuse-overlayfs
$ nerdctl run ...
```

## How to test

To run the test as a non-root user, [RootlessKit](https://github.com/rootless-containers/rootlesskit) needs to be installed.

```console
$ go test -exec rootlesskit -test.v -test.root
```

## Project details
fuse-overlayfs-snapshotter is a containerd **non-core** sub-project, licensed under the [Apache 2.0 license](./LICENSE).
As a containerd non-core sub-project, you will find the:
 * [Project governance](https://github.com/containerd/project/blob/master/GOVERNANCE.md),
 * [Maintainers](./MAINTAINERS),
 * and [Contributing guidelines](https://github.com/containerd/project/blob/master/CONTRIBUTING.md)

information in our [`containerd/project`](https://github.com/containerd/project) repository.
