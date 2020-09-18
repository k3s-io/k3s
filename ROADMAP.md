Roadmap
---
This represents the larger, bigger impact features and enhancements we have planned for K3s. There are many more tactical enhancements and fixes that can be found by perusing our [GitHub milestones](https://github.com/rancher/k3s/milestones).


v1.19 - End of August 2020
---
- Add support for config file based configuration
- Replace experimental embedded dqlite with embedded etcd
- Support ability to customize baked-in Helm charts
- Improve and refactor SELinux support
- Improve CentOS and RHEL 7 and 8 support


v1.20 - December 2020
---
- Support a 2-node hot/cold HA model by leveraging etcd's learner node feature
- Embedded container image registry
- Introduce `k3s build` feature for easier local image building
- Disentangle code and docs from Rancher Labs
- Reduce patch set K3s requires for Kubernetes
- Migrate to using upstream’s kubelet certificate rotation rather than our own custom logic
- Migrate to using the kubelet config file for configuring kubelets rather than command line args
- Upgrade to Traefik 2.x for ingress
- Use "staging" location for binaries as part of upgrades


Backlog
---
- Windows support
- Introduce "Cloud Provider" builds that have CSI and CCM built-in for each provider
- Improve low-power support
- Real-time operating system support
- Graduate encrypted networking support from experimental to GA
- Graduate network policy support from experimental to GA
