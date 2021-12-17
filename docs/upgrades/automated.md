---
title: "Automated Upgrades"
weight: 20
---

>**Note:** This feature is available as of [v1.17.4+k3s1](https://github.com/rancher/k3s/releases/tag/v1.17.4%2Bk3s1)

### Overview

You can manage K3s cluster upgrades using Rancher's system-upgrade-controller. This is a Kubernetes-native approach to cluster upgrades. It leverages a [custom resource definition (CRD)](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/#custom-resources), the `plan`, and a [controller](https://kubernetes.io/docs/concepts/architecture/controller/) that schedules upgrades based on the configured plans.

A plan defines upgrade policies and requirements. This documentation will provide plans with defaults appropriate for upgrading a K3s cluster. For more advanced plan configuration options, please review the [CRD](https://github.com/rancher/system-upgrade-controller/blob/master/pkg/apis/upgrade.cattle.io/v1/types.go).

The controller schedules upgrades by monitoring plans and selecting nodes to run upgrade [jobs](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/) on. A plan defines which nodes should be upgraded through a [label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/). When a job has run to completion successfully, the controller will label the node on which it ran accordingly.

>**Note:** The upgrade job that is launched must be highly privileged. It is configured with the following:
>
- Host `IPC`, `NET`, and `PID` namespaces
- The `CAP_SYS_BOOT` capability
- Host root mounted at `/host` with read and write permissions

For more details on the design and architecture of the system-upgrade-controller or its integration with K3s, see the following Git repositories:

- [system-upgrade-controller](https://github.com/rancher/system-upgrade-controller)
- [k3s-upgrade](https://github.com/rancher/k3s-upgrade)

To automate upgrades in this manner, you must do the following:

1. Install the system-upgrade-controller into your cluster
1. Configure plans

>**Note:** Users can and should use Rancher to upgrade their K3s cluster if Rancher is managing it. 
>
> * If you choose to use Rancher to upgrade, the following steps below are taken care of for you.
> * If you choose not to use Rancher to upgrade, you must use the following steps below to do so.


### Install the system-upgrade-controller
 The system-upgrade-controller can be installed as a deployment into your cluster. The deployment requires a service-account, clusterRoleBinding, and a configmap. To install these components, run the following command:
```
kubectl apply -f https://github.com/rancher/system-upgrade-controller/releases/latest/download/system-upgrade-controller.yaml
```
The controller can be configured and customized via the previously mentioned configmap, but the controller must be redeployed for the changes to be applied.


### Configure plans
It is recommended that you minimally create two plans: a plan for upgrading server (master) nodes and a plan for upgrading agent (worker) nodes. As needed, you can create additional plans to control the rollout of the upgrade across nodes. The following two example plans will upgrade your cluster to K3s v1.17.4+k3s1. Once the plans are created, the controller will pick them up and begin to upgrade your cluster.
```
# Server plan
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: server-plan
  namespace: system-upgrade
spec:
  concurrency: 1
  cordon: true
  nodeSelector:
    matchExpressions:
    - key: node-role.kubernetes.io/master
      operator: In
      values:
      - "true"
  serviceAccountName: system-upgrade
  upgrade:
    image: rancher/k3s-upgrade
  version: v1.17.4+k3s1
---
# Agent plan
apiVersion: upgrade.cattle.io/v1
kind: Plan
metadata:
  name: agent-plan
  namespace: system-upgrade
spec:
  concurrency: 1
  cordon: true
  nodeSelector:
    matchExpressions:
    - key: node-role.kubernetes.io/master
      operator: DoesNotExist
  prepare:
    args:
    - prepare
    - server-plan
    image: rancher/k3s-upgrade
  serviceAccountName: system-upgrade
  upgrade:
    image: rancher/k3s-upgrade
  version: v1.17.4+k3s1
```
There are a few important things to call out regarding these plans:

First, the plans must be created in the same namespace where the controller was deployed.

Second, the `concurrency` field indicates how many nodes can be upgraded at the same time. 

Third, the server-plan targets server nodes by specifying a label selector that selects nodes with the `node-role.kubernetes.io/master` label. The agent-plan targets agent nodes by specifying a label selector that select nodes without that label.

Fourth, the `prepare` step in the agent-plan will cause upgrade jobs for that plan to wait for the server-plan to complete before they execute.

Fifth, both plans have the `version` field set to v1.17.4+k3s1. Alternatively, you can omit the `version` field and set the `channel` field to a URL that resolves to a release of K3s. This will cause the controller to monitor that URL and upgrade the cluster any time it resolves to a new release. This works well with the [release channels](basic.md#release-channels). Thus, you can configure your plans with the following channel to ensure your cluster is always automatically upgraded to the newest stable release of K3s:
```
apiVersion: upgrade.cattle.io/v1
kind: Plan
...
spec:
  ...
  channel: https://update.k3s.io/v1-release/channels/stable

```

As stated, the upgrade will begin as soon as the controller detects that a plan was created. Updating a plan will cause the controller to re-evaluate the plan and determine if another upgrade is needed.

You can monitor the progress of an upgrade by viewing the plan and jobs via kubectl:
```
kubectl -n system-upgrade get plans -o yaml
kubectl -n system-upgrade get jobs -o yaml
```

