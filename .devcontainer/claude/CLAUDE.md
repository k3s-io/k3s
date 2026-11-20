# K3s dev container

You are running inside the K3s dev container (`.devcontainer/`), as root, in the K3s build
environment: the `infra` stage of the repository's `Dockerfile` (Alpine, the same Go toolchain and
`xx` cross-compiler as CI). The repository is at `/go/src/github.com/k3s-io/k3s`.

## Use `k3s-dev` for building, running and testing

- `k3s-dev build` runs `scripts/download` (once), `scripts/build` and `scripts/package-cli`.
  The result is `dist/artifacts/k3s`. A full first build takes a while; later builds are incremental.
- `k3s-dev reload` rebuilds and restarts the live cluster on the new binary. Use it to verify a
  change end to end. `k3s-dev status` shows whether the cluster runs a release or the local build.
- `k3s-dev logs` shows the K3s server log (`/var/log/k3s.log`). Check it when something fails.
- `k3s-dev test unit [PKG...]`, `k3s-dev test integration [NAME]`, `k3s-dev test docker [NAME]`
  wrap the commands in `tests/TESTING.md`. Prefer the narrowest package or suite that covers a change.
- `k3s-dev lint` runs golangci-lint on the changes since the merge base, like CI.
- `k3s-dev agent add N` adds agent nodes as Docker containers running the same build.

`kubectl`, `crictl` and `ctr` are symlinks to the K3s binary that runs the cluster, so they always
match it. `KUBECONFIG` is set. The `kubernetes` MCP server is registered and talks to this cluster.

## Project rules

- Read `CONTRIBUTING.md`. Every commit needs a DCO sign-off: `git commit -s`.
- Read `.agents/AGENTS.md` for the maintainers' agentic workflows; follow a workflow's steps and
  constraints when the task matches one.
- Run `go mod tidy` after changing `go.mod`.
- Never push to `k3s-io/k3s` directly. Changes go through a pull request from a fork.
- The dev cluster is disposable: `k3s-dev nuke` resets it. Never point these tools at a cluster
  outside this container without the user asking.
