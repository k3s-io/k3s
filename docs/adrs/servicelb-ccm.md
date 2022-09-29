# Move the ServiceLB load-balancer controller into the K3s cloud provider

Date: 2022-09-29

## Status

Accepted

## Context

K3s includes a stub cloud-provider the implements just enough node lifecycle functionality (the
`cloudprovider.Instances` interface) to get node addresses set properly, and clear the Uninitialized taint
that is added to nodes when they first join the cluster. The cloud-provider interface also has extension
points for load-balancer controllers, but we did not implement these, in favor of running a standalone
ServiceLB controller that is directly hooked into the core Wrangler controllers.

Because it doesn't make use of the existing load-balancer interface, the ServiceLB controller must implement
all the logic to watch Services, ensure that it's only handling services of the correct type and state, manage
finalizers, and so on.  This would all be handled by core Kubernetes code if we implemented the
`cloudprovider.LoadBalancer` interface.

## Decision

We will move the ServiceLB code into the cloud-controller, as a backend for the LoadBalancer interface
implementation. Existing behavior for disabling node lifecycle functionality will be retained, such that users
can still use ServiceLB alongside other cloud-controller-managers that handle node lifecycle. Support for
customizing ServiceLB behavior via node labels will be retained.

## Consequences

* K3s uses less resources when ServiceLB is disabled, as several core controllers are no longer started
unconditionally.
* The `--disable-cloud-controller` flag now disables the CCM's `cloud-node` and `cloud-node-lifecycle`
controllers that were historically the only supported controllers.
* The `--disable=servicelb` flag now disables the CCM's `service` controller.
* If the cloud-controller and servicelb are both disabled, the cloud-controller-manager is not run at all.
