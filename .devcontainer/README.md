# K3s Dev Container

Open this repository in a [Dev Container](https://containers.dev/) (VS Code, GitHub Codespaces or any
tool that supports the spec) and you get:

- **The CI build environment.** The container is built from the `infra` stage of the repository's
  [`Dockerfile`](../Dockerfile): the same Go version, `xx` cross-compiler and static libraries that
  `make` and CI use. When updatecli bumps Go in the Dockerfile, the dev container follows automatically.
- **A live K3s cluster that runs your code.** `k3s-dev build && k3s-dev reload` compiles this checkout
  and restarts the cluster on the new binary, keeping its state and pods, like an upgrade on a real node.
- **A debugger attached to the K3s server.** `k3s-dev debug`, then F5 in VS Code. Breakpoints work in
  K3s code and in the Kubernetes components it embeds.
- **Every test suite, one command each.** Unit, integration and `tests/docker`, run against your build.
- **Multi-node clusters.** `k3s-dev agent add 2` adds agent nodes running the same build.
- **AI that knows the project.** Claude Code is installed and briefed on `k3s-dev`, the DCO and the
  maintainers' workflows in [`.agents/`](../.agents/AGENTS.md). The Kubernetes MCP server is wired to
  your dev cluster for Claude Code and VS Code agent mode.
- **Headlamp, k9s, helm, kubectl, golangci-lint, gopls, delve, ginkgo, gh**, all pinned and
  checksum-verified.

```text
 _  _______        ____
| |/ /___ / ___   |  _ \  _____   __
| ' /  |_ \/ __|  | | | |/ _ \ \ / /
| . \ ___) \__ \  | |_| |  __/\ V /
|_|\_\____/|___/  |____/ \___| \_/

  Cluster      ● running  1 node(s) Ready
  K3s          v1.37.2+k3s-647ad670  your build from 647ad6706791
  Go           go1.26.10  (same toolchain as CI: Dockerfile, target infra)
  Headlamp     http://localhost:4466  (login token: k3s-dev token)
```

## Getting started

**GitHub Codespaces:** Code → Codespaces → Create codespace. Choose a machine with at least 4 cores
and 16 GB of memory (8 cores make the first build much faster). Without a prebuild, creating a
Codespace takes about 4 minutes while the image builds; if the browser stays on "Opening Remote..."
after that, reload the page. Repository admins can make this near-instant with a
[Codespaces prebuild](https://docs.github.com/en/codespaces/prebuilding-your-codespaces/configuring-prebuilds)
(Settings → Codespaces → Set up prebuild, trigger "On configuration change").

**VS Code:** install [Docker](https://docs.docker.com/get-docker/) (Docker Desktop, Rancher Desktop,
Colima or Docker Engine) and the
[Dev Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
extension, open the repository and click **Reopen in Container**.

**Any other tool:** `npx @devcontainers/cli up --workspace-folder .`

When the container starts, it boots the latest stable K3s release so you have a cluster within a
minute. Build your own and switch to it:

```bash
k3s-dev build     # first build: downloads modules and dependencies, later builds are incremental
k3s-dev reload    # rebuild and restart the cluster on dist/artifacts/k3s
k3s-dev status    # mode: dev, version: your commit
```

## `k3s-dev` commands

| Command | What it does |
|---|---|
| `k3s-dev up [--release [VERSION\|CHANNEL]] [-- ARGS]` | Start the server. Uses your build when `dist/artifacts/k3s` exists, otherwise the stable release. Arguments after `--` go to `k3s server`. |
| `k3s-dev build [--debug] [--download]` | `scripts/download` (first time only), `scripts/build` and `scripts/package-cli`, with the same `xx-go` wrapper as the Dockerfile's `build` stage. |
| `k3s-dev reload [--debug]` | `build`, then restart the cluster on the new binary. |
| `k3s-dev restart` | Restart the server process. Pods keep running. |
| `k3s-dev down` | Stop K3s and all pods, using the `k3s-killall.sh` embedded in [`install.sh`](../install.sh). State is kept. |
| `k3s-dev nuke` | Delete all cluster state. |
| `k3s-dev debug [--no-build]` | Debug build (`DEBUG=true`), reload, and attach delve on port 2345. |
| `k3s-dev agent add [N]` / `rm [NAME\|all]` / `ls` | Agent nodes as containers. For your build the image is built from [`package/Dockerfile`](../package/Dockerfile), like `make image`. |
| `k3s-dev test unit [PKG...]` | `go test ./pkg/... -run Unit`, see [`tests/TESTING.md`](../tests/TESTING.md). |
| `k3s-dev test integration [NAME]` | Integration tests against `dist/artifacts/k3s`. Stops the dev cluster first, because each test starts its own server. |
| `k3s-dev test docker [NAME]` | A [`tests/docker`](../tests/docker) suite (default `basics`) against your build's image. |
| `k3s-dev lint [--all]` | golangci-lint on the changes since your merge base, like CI. |
| `k3s-dev logs [-f]`, `status`, `wait`, `doctor`, `token` | Server log, state, readiness, `k3s check-config`, Headlamp token. |

`kubectl`, `crictl` and `ctr` are symlinks to the K3s binary that runs the cluster, so the client
always matches the server.

`make`, `make binary` and `make validate` also work, through the Docker daemon in the container.

## Debugging K3s

```bash
k3s-dev debug
```

This builds with `DEBUG=true` (optimisations off, symbols kept), restarts the cluster on that build
and attaches delve to the running server. In VS Code, open **Run and Debug** and start
**Attach to k3s server (k3s-dev debug)**. Set a breakpoint in, for example,
`pkg/server/server.go` or a controller under `pkg/`, and trigger it with `kubectl`. While the debugger
is paused, the API server is paused too.

The **Debug unit tests in current package** launch configuration debugs the `Unit` tests of the file
you have open.

## AI assistants

- **Claude Code** (`claude` in a terminal, or the VS Code extension). The container gives it a
  [`CLAUDE.md`](claude/CLAUDE.md) that explains `k3s-dev`, the DCO sign-off and the agentic workflows
  in [`.agents/`](../.agents/AGENTS.md). Your login is kept in a volume, so it survives rebuilds.
- **Kubernetes MCP server** ([containers/kubernetes-mcp-server](https://github.com/containers/kubernetes-mcp-server)),
  registered with Claude Code and with VS Code agent mode. It can inspect and change the dev cluster:
  *"Why is the traefik pod restarting?"*, *"Deploy nginx with 3 replicas and expose it"*.

Agents work in your fork and your name. The maintainers' rules in `.agents/AGENTS.md` apply: every
change goes through a pull request that you have reviewed.

## Headlamp

[Headlamp](https://headlamp.dev/), the Kubernetes SIG UI project, is installed by K3s itself: `k3s-dev up`
puts [`addons/headlamp.yaml`](addons/headlamp.yaml) in K3s's auto-deploy manifests directory and the
built-in helm-controller installs the chart. Open port 4466 and log in with `k3s-dev token`.
Set `K3S_DEV_ADDONS=none` to skip it.

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `K3S_DEV_SERVER_ARGS` | | Extra `k3s server` arguments, e.g. `--disable=traefik --cluster-init` |
| `K3S_DEV_CHANNEL` | `stable` | Release channel used when there is no local build |
| `K3S_DEV_ADDONS` | `headlamp` | Comma-separated files from `addons/`, or `none` |
| `K3S_DEV_DOCKER` | `true` | Start the Docker daemon in the container |

`/etc/rancher/k3s/config.yaml` works as on any K3s node. To share a variable between all open
terminals, use `kexport NAME=value` and `kunset NAME`.

From the host, the cluster is reachable on the forwarded port 6443 with the kubeconfig in `.kube/config`
(ignored by git): `KUBECONFIG=.kube/config kubectl get nodes`.

## How it works

```text
devcontainer.json
 ├─ build: ../Dockerfile, target "infra"     the K3s CI build environment (Alpine, Go, xx, static libs)
 ├─ features/k3s-dev                          pinned tools, cgroup v2 setup, Docker daemon, shell integration
 ├─ volumes                                   Go module and build caches (shared), cluster state, Docker, Claude
 └─ postStart: k3s-dev up                     boots the cluster
bin/k3s-dev                                   the commands above; edit them without rebuilding the container
bin/motd                                      the welcome banner
addons/                                       manifests k3s-dev drops into K3s's auto-deploy directory
claude/CLAUDE.md                              briefing for Claude Code inside the container
```

K3s runs directly in the container, not in a VM or in a nested `rancher/k3s` container. The container
is privileged. When K3s runs as PID 1 in the `rancher/k3s` image, it prepares cgroup v2 itself
(`EvacuateCgroup2` in [`pkg/cli/cmds/init_linux.go`](../pkg/cli/cmds/init_linux.go)). Here it is not
PID 1, so the feature's [`init.sh`](features/k3s-dev/init.sh) does the same before anything else starts.

The cluster, Docker and kubelet state live in volumes because overlayfs cannot be nested on the
container's own overlay filesystem.

### Updating tool versions

Versions and SHA256 checksums are at the top of [`features/k3s-dev/install.sh`](features/k3s-dev/install.sh).
Keep `GOLANGCI_LINT_VERSION` in step with `.github/workflows/validate.yaml`, and `GOPLS_VERSION`
compatible with the Go version in the Dockerfile. The Headlamp chart version is in
[`addons/headlamp.yaml`](addons/headlamp.yaml).

The [Dev Container workflow](../.github/workflows/devcontainer.yaml) builds the container, boots a
release cluster, builds the pull request's commit, reloads onto it, adds an agent node and runs a unit
test. It runs on pull requests that change the dev container, the Dockerfile or the build scripts.

## Troubleshooting

| Symptom | Fix |
|---|---|
| Banner says **stopped** | `k3s-dev up`, then `k3s-dev logs` if it stops again. |
| `k3s-dev build` is killed or very slow | Give Docker or the Codespace more memory (16 GB recommended). |
| Node not Ready after switching builds | `k3s-dev logs`. To start clean: `k3s-dev nuke && k3s-dev up`. |
| Port 6443 or 4466 already in use on the host | Stop the other cluster, or change `forwardPorts` in `devcontainer.json`. |
| `docker: Cannot connect to the Docker daemon` | `cat /var/log/dockerd.log`. |
| Files in `bin/`, `build/`, `dist/` owned by root (Linux hosts) | The container runs as root, like the Dockerfile build. `sudo chown -R $USER .` on the host, or `k3s-dev` from inside the container. |

`k3s-dev doctor` prints the environment and the relevant part of `k3s check-config`.

> [!WARNING]
> This is a development environment. The container is privileged, the cluster has default
> credentials and Headlamp logs in as cluster-admin. Don't expose its ports publicly.
