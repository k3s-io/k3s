# Security updates and versions bump automation

Date: 2022-11-24

## Status

Accepted

## Context

The current process of keeping third-party dependencies and container images up to date is manual and time consuming, which makes us lack behind upstream updates. This leads to K3s sometimes shipping outdated dependencies and images that can introduce security issues (CVEs) in our code and in users environments. A process is needed to automate the discovery of updates and PRs, so developers don't need to spend valuable time with manual tasks that can be easily automate.

The basic requirements that we need for such automation regarding the tooling are:

1. The tool is free and open-source.
2. Supports major packages ecosystems - Docker, Go, Python etc.
3. Automatically opens pull requests (PRs).
4. Preferably supports bumping versions in unstructured formats, e.g., `ENV` vars in Dockerfiles and by matching regular expressions.
5. Preferably supports scripting the automation process, e.g., update package A in repo B after package X in repo Y matches a pre-defined version criteria.

There are well known free and/or open-source tools available for this kind of automation:

### 1. [Dependabot](https://docs.github.com/en/code-security/dependabot)

- Provides PR automation for version and security updates.
- Is provided as a service by GitHub and free for public and private repos.
  - There is no need to add a GH app or token in the repo.
- Simple to configure in the repo settings in GitHub and with an YAML configuration `.github/dependabot.yml` file.
- Supports bumping of images in Dockerfiles and major packages ecosystem. See full list [here](https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file#package-ecosystem).
- Doesn't support:
  - Bumping versions in unstructured formats.
  - Orchestrating updates.

### 2. [Renovate](https://github.com/renovatebot/renovate)

- Is similar to Dependabot.
- Requires installation of [Renovate bot](https://github.com/apps/renovate) as a GH app.
- Is provided as a service by [Mend](https://www.mend.io/free-developer-tools/renovate/) (previously called WhiteSource).
- Is configured through a JSON file `renovate.json`.
- Supports bumping of images in Dockerfiles and major packages ecosystem. See full list [here](https://docs.renovatebot.com/golang/).
- Supports version bumps in unstructured formats through [data sources](https://docs.renovatebot.com/modules/datasource/).
- Doesn't support orchestrating updates.

### 3. [Updatecli](https://www.updatecli.io/)

- Is similar to Dependabot and Renovate.
- Is installed as a [GH Action](https://www.updatecli.io/docs/automate/github_action/).
  - Requires configuration of a GH token.
- Is fully open-source and a core maintainer is a current SUSE employee.
- All configurations must be scripted as an [YAML file](https://www.updatecli.io/docs/prologue/quick-start/).
  - For bumps in images and Go dependencies, for example, it's necessary to script the steps more verbosely when compared to Dependabot and Renovate.
- Supports version bumps in unstructured formats.
- Supports orchestrating updates through [`conditions`](https://www.updatecli.io/docs/core/condition/).
- It's being used in some SUSE projects - [Epinio](https://github.com/epinio/helm-charts/blob/e0cec6d31be78a418dbfb06efcc57f60385ec88f/updatecli/updatecli.d/epinio.yaml) and [Kubewarden](https://github.com/kubewarden/deprecated-api-versions-policy/blob/dabf594f6eac8143d13a859b2aa0279518d44b69/updatecli-manifest.yaml). Soon will also be implemented in Rancher.

Each tool has its strong points and a combination of them will be required.

| Features x Tool | Dependabot | Renovate | Updatecli |
| --------------- | ---------- | -------- | --------- |
| Provides pull request automation | ✅ | ✅ | ✅ |
| Minimal configuration required for bumping major packages ecosystems | ✅ | ✅ | ⚪ |
| Minimal integration required (no GH app or token) | ✅ | ⚪ | ⚪ |
| Supports version bumps in unstructured formats | ⚪ | ✅ | ✅ |
| Supports orchestrating updates | ⚪ | ⚪ | ✅ |
| Offers greater extensibility | ⚪ | ⚪ | ✅ |

## Decision

Based on the evaluated context, we decided to use Dependabot and Updatecli for automating version and security bumps.

- Dependabot for its simplicity to integrate with GitHub and covering the major packages ecosystem with minimal configuration required.
- Updatecli for allowing orchestration and automation of updates in unstructured formats.

## Consequences

- PRs for version bumps of dependencies are automated.
- PRs for security updates of dependencies are automated.
- Developers spend less time doing manual tasks.
- K3s code and images are shipped with fewer security issues in dependencies.
- Users benefit from greater security.
