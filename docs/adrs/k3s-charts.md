# Stage Helm charts through k3s-charts

Date: 2022-11-17

## Status

Accepted

## Context

The upstream Traefik chart repo has seen significant changes over the last month. Upstream has changed their repo structure, and actively removed content from deprecated locations,
at least twice. In both cases, this immediately broke K3s CI, requiring changes to our build scripts in order to restore the ability to build, test, and package K3s.

The K3s chart build process also makes several changes to the upstream chart to add values and break out the CRDs, using an ad-hoc set of scripts that are difficult to maintain.
There are better tools available to perform this same task, if we did so in a dedicated repo.

## Decision

We will make use of the [charts-build-scripts](https://github.com/rancher/charts-build-scripts) tool to customize the upstream chart and stage it through a stable intermediate repo.

## Consequences

When updating Helm charts distributed with K3s, additional pull requests will be necessary to stage new versions into the k3s-io/k3s-charts repo, before updating the chart version in K3s.
