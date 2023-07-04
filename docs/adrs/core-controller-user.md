# Use a dedicated user for K3s core controllers

Date: 2023-05-26

## Status

Accepted

## Context

Users who collect audit logs from K3s currently have a hard time determining if an action was performed by an administrator, or by the K3s supervisor.
This is due to the K3s supervisor using the same `system:admin` user for both the admin kubeconfig, and the kubeconfig used by core Wrangler controllers that drive core functionality and the deploy/helm controllers.

Users may have policies in place that prohibit use of the `system:admin` account, or that require service accounts to be distinct from user accounts.

## Decision

* We will add a new kubeconfig for the K3s supervisor controllers: core functionality, deploy (AddOns; aka the manifests directory), and helm (HelmChart/HelmChartConfig).
* Each of the three controllers will use a dedicated user-agent to further assist in discriminating between events, via both audit logs and resource ManageFields tracking.
* The new user account will use existing core Kubernetes group RBAC.

## Consequences

* K3s servers will create and manage an additional kubeconfig, client cert, and key that is intended only for use by the supervisor controllers.
* K3s supervisor controllers will use distinct user-agents to further discriminate between which component initiated the request.
