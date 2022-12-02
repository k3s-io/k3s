# Updatecli automation

*Note:* This automation is still work in progress and subject to change. For more information, please consult [PR #6559](https://github.com/k3s-io/k3s/pull/6559).

This project uses [Updatecli](https://github.com/updatecli/updatecli) to automate and orchestrate security related updates and versions bumps in K3s.

## Tool

We use Updatecli for this automation, instead of Dependabot or Renovate, because of its extensibility and multiple [plugins resources](https://www.updatecli.io/docs/prologue/introduction/) that allow greater flexibility when automating sequences of conditional update steps across multiple repos.

For detailed information on how to use Updatecli, please consult its [documentation](https://www.updatecli.io/docs/prologue/introduction/) page.

## Scope

The main usage of Updatecli is for:

* Bumping versions in unstructured formats, e.g., environment variables in Dockerfiles and by matching regular expressions.
* Scripting the automation process, e.g., update package A in repo B after package X in repo Y matches a pre-defined version criteria.

### Not in scope

* Updatecli will only open a pull request in the targeted repo. It's not responsible for approving and merging the PR.
* The resulting PR must still follow the rules of the targeted repo, e.g., passing checks, QA testing, review process etc.

## Project organization

A manifest or pipeline consists of three stages - source, condition and target - that define how to apply the update strategy.

When adding a new manifest, please follow the example structure defined below.

```
.
└── updatecli
    ├── scripts                            # Contains the auxiliary scripts used in the manifests
    ├── updatecli.d
    │   ├── golang-alpine.yaml             # Ideally each pipeline file corresponds to a dependency update
    │   ├── helm-controller.yaml
    │   ├── klipper.yaml
    └── values.yaml                        # Configuration values
```

## Local testing

Local testing of manifests require:

1. Updatecli binary that can be download from [updatecli/updatecli#releases](https://github.com/updatecli/updatecli/releases). Test only with the latest stable version.
   1. Always run locally with the command `diff`, that will show the changes without actually applying them.
2. A GitHub [PAT](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/creating-a-personal-access-token) (personal access token). The only required permission scope for Updatecli to work, when targeting only public repos, is `public_repo`.
   1. For obvious security reasons and to avoid leaking your GH PAT, export it as a local environment variable.

```shell
export UPDATECLI_GITHUB_TOKEN="your GH PAT"
updatecli diff --clean --config updatecli/updatecli.d/ --values updatecli/values.yaml            
```

## Contributing

Everyone is free to contribute with new manifests and pipelines for security version bumps targeting Rancher owned repos.

Before contributing, please follow the guidelines provided in this readme and make sure to test locally your changes before opening a PR.

