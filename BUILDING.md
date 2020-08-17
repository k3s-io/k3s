See the [release](https://github.com/rancher/k3s/releases/latest) page for pre-built releases.

The clone will be much faster on this repo if you do

```bash
git clone --depth 1 https://github.com/rancher/k3s.git
mkdir -p build/data && ./scripts/download && go generate
```

This repo includes all of Kubernetes history so `--depth 1` will avoid most of that.

To build the full release binary run `make` and that will create `./dist/artifacts/k3s`.

Optionally to build the binaries using without running linting (ie; if you have uncommitted changes):

```bash
SKIP_VALIDATE=true make
```

If you make any changes to go.mod and want to update the vendored modules, you should run the following before runnining `make`:
```bash
go mod vendor && go mod tidy
```

Kubernetes Source
-----------------

The source code for Kubernetes is in `vendor/` and the location from which that is copied
is in `./go.mod`.  Go to the referenced repo/tag and you'll find all the patches applied
to upstream Kubernetes.

