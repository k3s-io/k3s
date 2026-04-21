**Note:** In case you are looking for the pre-built releases see the [release page](https://github.com/k3s-io/k3s/releases/latest).

## Prerequisites

To build K3s locally, your environment must meet the following requirements:

* make: The build system uses a Makefile to orchestrate the process.
* Docker: K3s builds inside containers to ensure a consistent environment.
* BuildKit & Buildx: The build process requires the docker-buildx-plugin


## Build k3s from source

Before getting started, bear in mind that this repository includes all of Kubernetes history, so consider shallow cloning with (`--depth 1`) to speed up the process.

```bash
git clone --depth 1 https://github.com/k3s-io/k3s.git
```

To build the full release binary, you may now run `make`, which will create `./dist/artifacts/k3s`.

To build the binaries using `make` without running linting (i.e.: if you have uncommitted changes):

```bash
SKIP_VALIDATE=true make
```

In case you make any changes to [go.mod](go.mod), you should run `go mod tidy` before running `make`.