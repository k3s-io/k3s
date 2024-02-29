# Testing in K3s

Date: 2024-02-23

# Context

## Background

Currently, Testing in K3s is categorized into various types and spread across Github Actions and Drone CI. The types are as follows:

GitHub Actions:
- Unit Tests: For testing individual components and functions, following a "white box" approach.
- Integration Tests: Test functionalities across multiple packages, using "black box" testing.
- Smoke Tests: Simple tests to ensure basic functionality works as expected. Broken into:
    - Cgroup: Tests cgroupv2 support.
    - Snapshotter: tests btrfs and overlayfs snapshotter support.
    - Install tests: Tests the installation of K3s on various OSes.

Drone CI:
- Docker Tests: Run clusters in containers to test basic functionality. Broken into:
    - Basic Tests: Run clusters in containers to test basic functionality.
    - Sonobuoy Conformance Tests: Run clusters in containers to validate K8s conformance. Runs on multiple database backends.
- End-to-End (E2E) Tests: Cover multi-node configuration/administration.

- Performance Tests: Use Terraform to test large-scale deployments of K3s clusters. These were legacy tests and are never run in CI.

## Problems

- The current testing infrastructure is complex and fragmented, leading to maintenance overhead. Not all testing is grouped inside the [tests directory](../../tests/).
- GitHub Actions had limited resources, making it unsuitable for running larger tests.
- GitHub Actions only supported hardware virtualiztion on Mac runners and that support was often broken.
- Drone CI cannot handle individual testing failures. If a single test fails, the entire build is marked as failed.

## New Developments

As of late January 2024, GitHub Actions has made significant improvements:
- The resources available to open source GitHub Actions have been doubled, with 4 CPU cores and 16GB of RAM. See blog post [here](https://github.blog/2024-01-17-github-hosted-runners-double-the-power-for-open-source/).
- Standard (i.e. free) Linux runners now support Nested Virtualization

## Decision

We will move towards a single testing platform, GitHub Actions, and leverage the recent improvements in resources and nested virtualization support. This will involve the following changes:

- Test distribution based on size and complexity:
    - Unit, Integration: Will continue to run in GitHub Actions due to their smaller scale and faster execution times.
    - Install Test, Docker Basic, and E2E Tests: Will run in GitHub Actions on standard linux runners thanks to recent enhancements.
    - Docker Conformance and large E2E Tests (2+ nodes): Still utilize Drone CI for resource-intensive scenarios.

- Consolidating all testing-related files within the "tests" directory for better organization and clarity.
- Cgroup smoke tests will be removed. As multiple Operating Systems now support CgroupV2 by default, these tests are no longer relevant.
- Snapshotter smoke test will be converted into a full E2E test.
- Remove of old performance tests, as they are no longer relevant. Scale testing is already handled by QA as needed. 

Tracking these changes is with [this issue](https://github.com/k3s-io/k3s/issues/9477)

## Consequences

- The testing infrastructure will be more organized and easier to maintain.
- The move to GitHub Actions will allow for faster feedback on PRs and issues.
- The removal of old tests will reduce the maintenance overhead.
- New testing process can be used as a model for related projects.

