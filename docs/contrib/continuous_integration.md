# Continuous Integration and Automation

Every change on the K3s repository, either made through a pull request or direct push, triggers the continuous integration pipelines defined within the same repository. Needless to say, all the K3s contributions can be merged until all the checks pass (AKA having green builds).

- [CI Platforms](#ci-platforms)
  - [GitHub Actions](#github-actions)
  - [Drone Pipelines](#drone-pipelines)
- [Running locally](#running-locally)

## CI Platforms

Currently, there are two different platforms involved in running the CI processes:

- GitHub actions
- Drone pipelines on CNCF infrastructure

### GitHub Actions

All the existing GitHub Actions are defined as YAML files under the `.github/workflows` directory. These can be grouped into:

- **PR Checks**. These actions run all the required validations upon PR creation and update. Covering the DCO compliance check, `x86_64` test batteries (unit, integration, smoke), and code coverage.
- **Repository automation**. Currently, it only covers issues and epic grooming.

Everything runs on GitHub's provided runners; thus, the tests are limited to run in `x86_64` architectures.

### Drone Pipelines

The Drone pipelines are defined in the `.drone.yml` file. These are designed to be run on separated clusters depending  on their end goal, being categorized into:

- **Continuous Integration**. It runs code linting and test batteries for x86_64, arm64, and arm architectures.
- **Publish Artifacts**. It runs the same checks as the CI one but adds the [fossa scan](https://fossa.com/) and the K3s artifacts building, packaging, and publishing steps. This cluster is involved mainly in K3s releases.

The K3s contributors do not need access to these drone clusters to get their PR's built. Still, for reference, these are their corresponding URLs <drone-pr.k3s.io> and <drone-publish.k3s.io>.

## Running locally

A contributor should verify their changes locally to speed up the pull request process. Fortunately, all the CI steps can be on local environments, except for the publishing ones, through either of the following methods:

- **Drone local execution**. The drone CLI allows the user to execute a specific pipeline locally through the `drone exec --pipeline <pipeline name>` command. This requires having Docker with volume mount support.
- **Dapper**. The [dapper tool](https://github.com/rancher/dapper) is a "Docker wrapper" that enables the user to run a set of instructions within a Docker container without relying on volume mounts. This tool is always used when any K3s [Makefile](../../Makefile) goal is invoked, which use as an entrypoint for the CI scripts.
- **Step direct invocation**. The CI steps invoked on the drone pipelines are defined in the scripts directory, which can be executed individually in the local environment. Worth noting, these scripts rely on GNU utils, which are not the same as the BSD ones installed on macOS environments, although they have the same name.

As mentioned above, the scripts within the `scripts` directory are the core of the CI process, being the most relevant ones:

- **validate**. Executes the `go generate` command and the linting tools, then asserts the K3s components versions.
- **test**. Triggers the unit and integration test batteries. This is further elaborated in the [TESTING.md](../../tests/TESTING.md) document.
- **build**. Builds all the K3s binaries.
- **package**. Triggers the packaging processes for K3s binaries and airgap images.
- **ci**. Use as the CI scripts entrypoint, orchestrating all the required steps.
- **clean**. Remove built binaries and packages.
