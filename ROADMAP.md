Roadmap
---
This represents the larger, bigger impact features and enhancements we have planned for K3s. There are many more tactical enhancements and fixes that can be found by perusing our [GitHub milestones](https://github.com/k3s-io/k3s/milestones).


v1.20 - December 2020
---
- Join CNCF (Woo hoo!)
- Migrate from rancher org to neutral k3s-io org
- Disentangle code and docs from Rancher Labs

v1.21 - TBD
---
- Introduce support for Cilium CNI
- Introduce `k3s build` feature for easier local image building
- Reduce patch set K3s requires for Kubernetes
- Migrate to using upstream’s kubelet certificate rotation rather than our own custom logic
- Migrate to using the kubelet config file for configuring kubelets rather than command line args
- Upgrade to Traefik 2.x for ingress
- Use "staging" location for binaries as part of upgrades

Backlog
---
- Support a 2-node hot/cold HA model by leveraging etcd's learner node feature
- Embedded container image registry
- Windows support
- Introduce "Cloud Provider" builds that have CSI and CCM built-in for each provider
- Improve low-power support
- Real-time operating system support
- Graduate encrypted networking support from experimental to GA
- Graduate network policy support from experimental to GA
