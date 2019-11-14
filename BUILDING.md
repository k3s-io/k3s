See the [release](https://github.com/rancher/k3s/releases/latest) page for pre-built releases.

The clone will be much faster on this repo if you do

    git clone --depth 1 https://github.com/rancher/k3s.git

This repo includes all of Kubernetes history so `--depth 1` will avoid most of that.

To build the full release binary run `make` and that will create `./dist/artifacts/k3s`.

Optionally to build the binaries using local go environment without running linting or building docker images:
```sh
./scripts/download && ./scripts/build && ./scripts/package-cli
```

For development, you just need go 1.12+ and a proper GOPATH.  To compile the binaries run:
```bash
go build -o k3s
go build -o kubectl ./cmd/kubectl
go build -o hyperkube ./vendor/k8s.io/kubernetes/cmd/hyperkube
```

This will create the main executable at `./dist/artifacts` , but it does not include the dependencies like containerd, CNI,
etc.  To run a server and agent with all the dependencies for development run the following
helper scripts:
```bash
# Server
./scripts/dev-server.sh

# Agent
./scripts/dev-agent.sh
```

Kubernetes Source
-----------------

The source code for Kubernetes is in `vendor/` and the location from which that is copied
is in `./go.mod`.  Go to the referenced repo/tag and you'll find all the patches applied
to upstream Kubernetes.
