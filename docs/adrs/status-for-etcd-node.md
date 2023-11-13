# A way for seeing the status of the etcd node

Date: 2023-09-14

## Status

Decided

## Context

It is difficult for a user to see if the etcd status has changed from learner to voter. 
As a result, there is a need for a controller or condition to make it easier for the user to view the status of their etcd node and how it is running.

One issue with not having this controller or condition is that when a cluster is provisioned with scaling, 
it's quite possible for the cluster to break due to quickly adding or removing a node for any reason.

With this feature, the user will be able to have a better understanding of the etcd status for each node, thus avoiding problems when provisioning clusters.

## Decision

We decided to add a status flag on our etcd controller.

## Consequences

Good:
- Better view of the etcd status