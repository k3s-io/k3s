---
title: "CIS Self Assessment Guide"
weight: 90
---


### CIS Kubernetes Benchmark v1.5 - K3s v1.17, v1.18, & v1.19

#### Overview

This document is a companion to the K3s security hardening guide. The hardening guide provides prescriptive guidance for hardening a production installation of K3s, and this benchmark guide is meant to help you evaluate the level of security of the hardened cluster against each control in the CIS Kubernetes benchmark. It is to be used by K3s operators, security teams, auditors, and decision-makers.

This guide is specific to the **v1.17**, **v1.18**, and **v1.19** release line of K3s and the **v1.5.1** release of the CIS Kubernetes Benchmark.

For more detail about each control, including more detailed descriptions and remediations for failing tests, you can refer to the corresponding section of the CIS Kubernetes Benchmark v1.5. You can download the benchmark after logging in to [CISecurity.org](https://www.cisecurity.org/benchmark/kubernetes/).

#### Testing controls methodology

Each control in the CIS Kubernetes Benchmark was evaluated against a K3s cluster that was configured according to the accompanying hardening guide.

Where control audits differ from the original CIS benchmark, the audit commands specific to K3s are provided for testing.

These are the possible results for each control:

- **Pass** - The K3s cluster under test passed the audit outlined in the benchmark.
- **Not Applicable** - The control is not applicable to K3s because of how it is designed to operate. The remediation section will explain why this is so.
- **Not Scored - Operator Dependent** - The control is not scored in the CIS benchmark and it depends on the cluster's use case or some other factor that must be determined by the cluster operator. These controls have been evaluated to ensure K3s does not prevent their implementation, but no further configuration or auditing of the cluster under test has been performed.

This guide makes the assumption that K3s is running as a Systemd unit. Your installation may vary and will require you to adjust the "audit" commands to fit your scenario.

### Controls

---
## 1 Master Node Security Configuration
### 1.1 Master Node Configuration Files

#### 1.1.1
Ensure that the API server pod specification file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The API server pod specification file controls various parameters that set the behavior of the API server. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Not Applicable


#### 1.1.2
Ensure that the API server pod specification file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The API server pod specification file controls various parameters that set the behavior of the API server. You should set its file ownership to maintain the integrity of the file. The file should be owned by `root:root`.
</details>

**Result:** Not Applicable


#### 1.1.3
Ensure that the controller manager pod specification file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The controller manager pod specification file controls various parameters that set the behavior of the Controller Manager on the master node. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Not Applicable


#### 1.1.4
Ensure that the controller manager pod specification file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The controller manager pod specification file controls various parameters that set the behavior of various components of the master node. You should set its file ownership to maintain the integrity of the file. The file should be owned by root:root.
</details>

**Result:** Not Applicable


#### 1.1.5
Ensure that the scheduler pod specification file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The scheduler pod specification file controls various parameters that set the behavior of the Scheduler service in the master node. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Not Applicable


#### 1.1.6
Ensure that the scheduler pod specification file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The scheduler pod specification file controls various parameters that set the behavior of the kube-scheduler service in the master node. You should set its file ownership to maintain the integrity of the file. The file should be owned by root:root.
</details>

**Result:** Not Applicable


#### 1.1.7
Ensure that the etcd pod specification file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The etcd pod specification file /var/lib/rancher/k3s/agent/pod-manifests/etcd.yaml controls various parameters that set the behavior of the etcd service in the master node. etcd is a highly- available key-value store which Kubernetes uses for persistent storage of all of its REST API object. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Not Applicable


#### 1.1.8
Ensure that the etcd pod specification file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The etcd pod specification file /var/lib/rancher/k3s/agent/pod-manifests/etcd.yaml controls various parameters that set the behavior of the etcd service in the master node. etcd is a highly- available key-value store which Kubernetes uses for persistent storage of all of its REST API object. You should set its file ownership to maintain the integrity of the file. The file should be owned by root:root.
</details>

**Result:** Not Applicable


#### 1.1.9
Ensure that the Container Network Interface file permissions are set to 644 or more restrictive (Not Scored)
<details>
<summary>Rationale</summary>
Container Network Interface provides various networking options for overlay networking. You should consult their documentation and restrict their respective file permissions to maintain the integrity of those files. Those files should be writable by only the administrators on the system.
</details>

**Result:** Not Applicable


#### 1.1.10
Ensure that the Container Network Interface file ownership is set to root:root (Not Scored)
<details>
<summary>Rationale</summary>
Container Network Interface provides various networking options for overlay networking. You should consult their documentation and restrict their respective file permissions to maintain the integrity of those files. Those files should be owned by root:root.
</details>

**Result:** Not Applicable


#### 1.1.11
Ensure that the etcd data directory permissions are set to 700 or more restrictive (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. This data directory should be protected from any unauthorized reads or writes. It should not be readable or writable by any group members or the world.
</details>

**Result:** Pass

**Audit:**
```bash
stat -c %a /var/lib/rancher/k3s/server/db/etcd
700
```

**Remediation:**
K3s manages the etcd data directory and sets its permissions to 700. No manual remediation needed. (only relevant when Etcd is used for the data store)


#### 1.1.12
Ensure that the etcd data directory ownership is set to `etcd:etcd` (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. This data directory should be protected from any unauthorized reads or writes. It should be owned by etcd:etcd.
</details>

**Result:** Not Applicable


#### 1.1.13
Ensure that the `admin.conf` file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The admin.conf is the administrator kubeconfig file defining various settings for the administration of the cluster. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.

In K3s, this file is located at `/var/lib/rancher/k3s/server/cred/admin.kubeconfig`.
</details>

**Result:** Pass

**Remediation:**
By default, K3s creates the directory and files with the expected permissions of `644`. No manual remediation should be necessary.


#### 1.1.14
Ensure that the `admin.conf` file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The admin.conf file contains the admin credentials for the cluster. You should set its file ownership to maintain the integrity of the file. The file should be owned by root:root.

In K3s, this file is located at `/var/lib/rancher/k3s/server/cred/admin.kubeconfig`.
</details>

**Result:** Pass

**Remediation:**
By default, K3s creates the directory and files with the expected ownership of `root:root`. No manual remediation should be necessary.


#### 1.1.15
Ensure that the `scheduler.conf` file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>

The scheduler.conf file is the kubeconfig file for the Scheduler. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.

In K3s, this file is located at `/var/lib/rancher/k3s/server/cred/scheduler.kubeconfig`.
</details>

**Result:** Pass

**Remediation:**
By default, K3s creates the directory and files with the expected permissions of `644`. No manual remediation should be necessary.


#### 1.1.16
Ensure that the `scheduler.conf` file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The scheduler.conf file is the kubeconfig file for the Scheduler. You should set its file ownership to maintain the integrity of the file. The file should be owned by root:root.

In K3s, this file is located at `/var/lib/rancher/k3s/server/cred/scheduler.kubeconfig`.
</details>

**Result:** Pass

**Remediation:**
By default, K3s creates the directory and files with the expected ownership of `root:root`. No manual remediation should be necessary.


#### 1.1.17
Ensure that the `controller.kubeconfig` file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The controller.kubeconfig file is the kubeconfig file for the Scheduler. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.

In K3s, this file is located at `/var/lib/rancher/k3s/server/cred/controller.kubeconfig`.
</details>

**Result:** Pass

**Remediation:**
By default, K3s creates the directory and files with the expected permissions of `644`. No manual remediation should be necessary.


#### 1.1.18
Ensure that the `controller.kubeconfig` file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The controller.kubeconfig file is the kubeconfig file for the Scheduler. You should set its file ownership to maintain the integrity of the file. The file should be owned by root:root.

In K3s, this file is located at `/var/lib/rancher/k3s/server/cred/controller.kubeconfig`.
</details>

**Result:** Pass

**Remediation:**
By default, K3s creates the directory and files with the expected ownership of `root:root`. No manual remediation should be necessary.


#### 1.1.19
Ensure that the Kubernetes PKI directory and file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
Kubernetes makes use of a number of certificates as part of its operation. You should set the ownership of the directory containing the PKI information and all files in that directory to maintain their integrity. The directory and files should be owned by root:root.
</details>

**Result:** Pass

**Audit:**
```bash
stat -c %U:%G /var/lib/rancher/k3s/server/tls
root:root
```

**Remediation:**
By default, K3s creates the directory and files with the expected ownership of `root:root`. No manual remediation should be necessary.


#### 1.1.20
Ensure that the Kubernetes PKI certificate file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
Kubernetes makes use of a number of certificate files as part of the operation of its components. The permissions on these files should be set to 644 or more restrictive to protect their integrity.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
stat -c %n\ %a /var/lib/rancher/k3s/server/tls/*.crt
```

Verify that the permissions are `644` or more restrictive.

**Remediation:**
By default, K3s creates the files with the expected permissions of `644`. No manual remediation is needed.


#### 1.1.21
Ensure that the Kubernetes PKI key file permissions are set to `600` (Scored)
<details>
<summary>Rationale</summary>
Kubernetes makes use of a number of key files as part of the operation of its components. The permissions on these files should be set to 600 to protect their integrity and confidentiality.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
stat -c %n\ %a /var/lib/rancher/k3s/server/tls/*.key
```

Verify that the permissions are `600` or more restrictive.

**Remediation:**
By default, K3s creates the files with the expected permissions of `600`. No manual remediation is needed.


### 1.2 API Server
This section contains recommendations relating to API server configuration flags


#### 1.2.1
Ensure that the `--anonymous-auth` argument is set to false (Not Scored)

<details>
<summary>Rationale</summary>
When enabled, requests that are not rejected by other configured authentication methods are treated as anonymous requests. These requests are then served by the API server. You should rely on authentication to authorize access and disallow anonymous requests.

If you are using RBAC authorization, it is generally considered reasonable to allow anonymous access to the API Server for health checks and discovery purposes, and hence this recommendation is not scored. However, you should consider whether anonymous discovery is an acceptable risk for your purposes.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "anonymous-auth"
```

Verify that `--anonymous-auth=false` is present.

**Remediation:**
By default, K3s kube-apiserver is configured to run with this flag and value. No manual remediation is needed.

#### 1.2.2
Ensure that the `--basic-auth-file` argument is not set (Scored)
<details>
<summary>Rationale</summary>
Basic authentication uses plaintext credentials for authentication. Currently, the basic authentication credentials last indefinitely, and the password cannot be changed without restarting the API server. The basic authentication is currently supported for convenience. Hence, basic authentication should not be used.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "basic-auth-file"
```

Verify that the `--basic-auth-file` argument does not exist.

**Remediation:**
By default, K3s does not run with basic authentication enabled. No manual remediation is needed.


#### 1.2.3
Ensure that the `--token-auth-file` parameter is not set (Scored)

<details>
<summary>Rationale</summary>
The token-based authentication utilizes static tokens to authenticate requests to the apiserver. The tokens are stored in clear-text in a file on the apiserver, and cannot be revoked or rotated without restarting the apiserver. Hence, do not use static token-based authentication.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "token-auth-file"
```

Verify that the `--token-auth-file` argument does not exist.

**Remediation:**
By default, K3s does not run with basic authentication enabled. No manual remediation is needed.

#### 1.2.4
Ensure that the `--kubelet-https` argument is set to true (Scored)

<details>
<summary>Rationale</summary>
Connections from apiserver to kubelets could potentially carry sensitive data such as secrets and keys. It is thus important to use in-transit encryption for any communication between the apiserver and kubelets.
</details>

**Result:** Not Applicable

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "kubelet-https"
```

Verify that the `--kubelet-https` argument does not exist.

**Remediation:**
By default, K3s kube-apiserver doesn't run with the `--kubelet-https` parameter as it runs with TLS. No manual remediation is needed.

#### 1.2.5
Ensure that the `--kubelet-client-certificate` and `--kubelet-client-key` arguments are set as appropriate (Scored)

<details>
<summary>Rationale</summary>
The apiserver, by default, does not authenticate itself to the kubelet's HTTPS endpoints. The requests from the apiserver are treated anonymously. You should set up certificate-based kubelet authentication to ensure that the apiserver authenticates itself to kubelets when submitting requests.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep -E 'kubelet-client-certificate|kubelet-client-key'
```

Verify that the `--kubelet-client-certificate` and `--kubelet-client-key` arguments exist and they are set as appropriate.

**Remediation:**
By default, K3s kube-apiserver is ran with these arguments for secure communication with kubelet. No manual remediation is needed.


#### 1.2.6
Ensure that the `--kubelet-certificate-authority` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
The connections from the apiserver to the kubelet are used for fetching logs for pods, attaching (through kubectl) to running pods, and using the kubelet’s port-forwarding functionality. These connections terminate at the kubelet’s HTTPS endpoint. By default, the apiserver does not verify the kubelet’s serving certificate, which makes the connection subject to man-in-the-middle attacks, and unsafe to run over untrusted and/or public networks.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "kubelet-certificate-authority"
```

Verify that the `--kubelet-certificate-authority` argument exists and is set as appropriate.

**Remediation:**
By default, K3s kube-apiserver is ran with this argument for secure communication with kubelet. No manual remediation is needed.


#### 1.2.7
Ensure that the `--authorization-mode` argument is not set to `AlwaysAllow` (Scored)
<details>
<summary>Rationale</summary>
The API Server, can be configured to allow all requests. This mode should not be used on any production cluster.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "authorization-mode"
```

Verify that the argument value doesn't contain `AlwaysAllow`.

**Remediation:**
By default, K3s sets `Node,RBAC` as the parameter to the `--authorization-mode` argument. No manual remediation is needed.


#### 1.2.8
Ensure that the `--authorization-mode` argument includes `Node` (Scored)
<details>
<summary>Rationale</summary>
The Node authorization mode only allows kubelets to read Secret, ConfigMap, PersistentVolume, and PersistentVolumeClaim objects associated with their nodes.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "authorization-mode"
```

Verify `Node` exists as a parameter to the argument.

**Remediation:**
By default, K3s sets `Node,RBAC` as the parameter to the `--authorization-mode` argument. No manual remediation is needed.


#### 1.2.9
Ensure that the `--authorization-mode` argument includes `RBAC` (Scored)
<details>
<summary>Rationale</summary>
Role Based Access Control (RBAC) allows fine-grained control over the operations that different entities can perform on different objects in the cluster. It is recommended to use the RBAC authorization mode.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "authorization-mode"
```

Verify `RBAC` exists as a parameter to the argument.

**Remediation:**
By default, K3s sets `Node,RBAC` as the parameter to the `--authorization-mode` argument. No manual remediation is needed.


#### 1.2.10
Ensure that the admission control plugin EventRateLimit is set (Not Scored)
<details>
<summary>Rationale</summary>
Using `EventRateLimit` admission control enforces a limit on the number of events that the API Server will accept in a given time slice. A misbehaving workload could overwhelm and DoS the API Server, making it unavailable. This particularly applies to a multi-tenant cluster, where there might be a small percentage of misbehaving tenants which could have a significant impact on the performance of the cluster overall. Hence, it is recommended to limit the rate of events that the API server will accept.

Note: This is an Alpha feature in the Kubernetes 1.15 release.
</details>

**Result:** **Not Scored - Operator Dependent**

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "enable-admission-plugins"
```

Verify that the `--enable-admission-plugins` argument is set to a value that includes EventRateLimit.

**Remediation:**
By default, K3s only sets `NodeRestriction,PodSecurityPolicy` as the parameter to the `--enable-admission-plugins` argument.
To configure this, follow the Kubernetes documentation and set the desired limits in a configuration file. Then refer to K3s's documentation to see how to supply additional api server configuration via the kube-apiserver-arg parameter.


#### 1.2.11
Ensure that the admission control plugin `AlwaysAdmit` is not set (Scored)
<details>
<summary>Rationale</summary>
Setting admission control plugin AlwaysAdmit allows all requests and do not filter any requests.

The AlwaysAdmit admission controller was deprecated in Kubernetes v1.13. Its behavior was equivalent to turning off all admission controllers.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "enable-admission-plugins"
```

Verify that if the `--enable-admission-plugins` argument is set, its value does not include `AlwaysAdmit`.

**Remediation:**
By default, K3s only sets `NodeRestriction,PodSecurityPolicy` as the parameter to the `--enable-admission-plugins` argument. No manual remediation needed.


#### 1.2.12
Ensure that the admission control plugin AlwaysPullImages is set (Not Scored)
<details>
<summary>Rationale</summary>
Setting admission control policy to `AlwaysPullImages` forces every new pod to pull the required images every time. In a multi-tenant cluster users can be assured that their private images can only be used by those who have the credentials to pull them. Without this admission control policy, once an image has been pulled to a node, any pod from any user can use it simply by knowing the image’s name, without any authorization check against the image ownership. When this plug-in is enabled, images are always pulled prior to starting containers, which means valid credentials are required.

</details>

**Result:** **Not Scored - Operator Dependent**

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "enable-admission-plugins"
```

Verify that the `--enable-admission-plugins` argument is set to a value that includes `AlwaysPullImages`.

**Remediation:**
By default, K3s only sets `NodeRestriction,PodSecurityPolicy` as the parameter to the `--enable-admission-plugins` argument.
To configure this, follow the Kubernetes documentation and set the desired limits in a configuration file. Then refer to K3s's documentation to see how to supply additional api server configuration via the kube-apiserver-arg parameter.

#### 1.2.13
Ensure that the admission control plugin SecurityContextDeny is set if PodSecurityPolicy is not used (Not Scored)
<details>
<summary>Rationale</summary>
SecurityContextDeny can be used to provide a layer of security for clusters which do not have PodSecurityPolicies enabled.
</details>

**Result:** Not Scored

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "enable-admission-plugins"
```

Verify that the `--enable-admission-plugins` argument is set to a value that includes `SecurityContextDeny`, if `PodSecurityPolicy` is not included.

**Remediation:**
K3s would need to have the `SecurityContextDeny` admission plugin enabled by passing it as an argument to K3s. `--kube-apiserver-arg='enable-admission-plugins=SecurityContextDeny`


#### 1.2.14
Ensure that the admission control plugin `ServiceAccount` is set (Scored)
<details>
<summary>Rationale</summary>
When you create a pod, if you do not specify a service account, it is automatically assigned the `default` service account in the same namespace. You should create your own service account and let the API server manage its security tokens.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "ServiceAccount"
```

Verify that the `--disable-admission-plugins` argument is set to a value that does not includes `ServiceAccount`.

**Remediation:**
By default, K3s does not use this argument. If there's a desire to use this argument, follow the documentation and create ServiceAccount objects as per your environment. Then refer to K3s's documentation to see how to supply additional api server configuration via the kube-apiserver-arg parameter.


#### 1.2.15
Ensure that the admission control plugin `NamespaceLifecycle` is set (Scored)
<details>
<summary>Rationale</summary>
Setting admission control policy to `NamespaceLifecycle` ensures that objects cannot be created in non-existent namespaces, and that namespaces undergoing termination are not used for creating the new objects. This is recommended to enforce the integrity of the namespace termination process and also for the availability of the newer objects.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "disable-admission-plugins"
```

Verify that the `--disable-admission-plugins` argument is set to a value that does not include `NamespaceLifecycle`.

**Remediation:**
By default, K3s does not use this argument. No manual remediation needed.


#### 1.2.16
Ensure that the admission control plugin `PodSecurityPolicy` is set (Scored)
<details>
<summary>Rationale</summary>
A Pod Security Policy is a cluster-level resource that controls the actions that a pod can perform and what it has the ability to access. The `PodSecurityPolicy` objects define a set of conditions that a pod must run with in order to be accepted into the system. Pod Security Policies are comprised of settings and strategies that control the security features a pod has access to and hence this must be used to control pod access permissions.

**Note:** When the PodSecurityPolicy admission plugin is in use, there needs to be at least one PodSecurityPolicy in place for ANY pods to be admitted. See section 1.7 for recommendations on PodSecurityPolicy settings.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "enable-admission-plugins"
```

Verify that the `--enable-admission-plugins` argument is set to a value that includes `PodSecurityPolicy`.

**Remediation:**
K3s would need to have the `PodSecurityPolicy` admission plugin enabled by passing it as an argument to K3s. `--kube-apiserver-arg='enable-admission-plugins=PodSecurityPolicy`.


#### 1.2.17
Ensure that the admission control plugin `NodeRestriction` is set (Scored)
<details>
<summary>Rationale</summary>
Using the `NodeRestriction` plug-in ensures that the kubelet is restricted to the `Node` and `Pod` objects that it could modify as defined. Such kubelets will only be allowed to modify their own `Node` API object, and only modify `Pod` API objects that are bound to their node.

</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "enable-admission-plugins"
```

Verify that the `--enable-admission-plugins` argument is set to a value that includes `NodeRestriction`.

**Remediation:**
K3s would need to have the `NodeRestriction` admission plugin enabled by passing it as an argument to K3s. `--kube-apiserver-arg='enable-admission-plugins=NodeRestriction`.


#### 1.2.18
Ensure that the `--insecure-bind-address` argument is not set (Scored)
<details>
<summary>Rationale</summary>
If you bind the apiserver to an insecure address, basically anyone who could connect to it over the insecure port, would have unauthenticated and unencrypted access to your master node. The apiserver doesn't do any authentication checking for insecure binds and traffic to the Insecure API port is not encrpyted, allowing attackers to potentially read sensitive data in transit.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "insecure-bind-address"
```

Verify that the `--insecure-bind-address` argument does not exist.

**Remediation:**
By default, K3s explicitly excludes the use of the `--insecure-bind-address` parameter. No manual remediation is needed.


#### 1.2.19
Ensure that the `--insecure-port` argument is set to `0` (Scored)
<details>
<summary>Rationale</summary>
Setting up the apiserver to serve on an insecure port would allow unauthenticated and unencrypted access to your master node. This would allow attackers who could access this port, to easily take control of the cluster.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "insecure-port"
```

Verify that the `--insecure-port` argument is set to `0`.

**Remediation:**
By default, K3s starts the kube-apiserver process with this argument's parameter set to `0`. No manual remediation is needed.


#### 1.2.20
Ensure that the `--secure-port` argument is not set to `0` (Scored)
<details>
<summary>Rationale</summary>
The secure port is used to serve https with authentication and authorization. If you disable it, no https traffic is served and all traffic is served unencrypted.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "secure-port"
```

Verify that the `--secure-port` argument is either not set or is set to an integer value between 1 and 65535.

**Remediation:**
By default, K3s sets the parameter of 6444 for the `--secure-port` argument. No manual remediation is needed.


#### 1.2.21
Ensure that the `--profiling` argument is set to `false` (Scored)
<details>
<summary>Rationale</summary>
Profiling allows for the identification of specific performance bottlenecks. It generates a significant amount of program data that could potentially be exploited to uncover system and program details. If you are not experiencing any bottlenecks and do not need the profiler for troubleshooting purposes, it is recommended to turn it off to reduce the potential attack surface.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "profiling"
```

Verify that the `--profiling` argument is set to false.

**Remediation:**
By default, K3s sets the `--profiling` flag parameter to false. No manual remediation needed.


#### 1.2.22
Ensure that the `--audit-log-path` argument is set (Scored)
<details>
<summary>Rationale</summary>
Auditing the Kubernetes API Server provides a security-relevant chronological set of records documenting the sequence of activities that have affected system by individual users, administrators or other components of the system. Even though currently, Kubernetes provides only basic audit capabilities, it should be enabled. You can enable it by setting an appropriate audit log path.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "audit-log-path"
```

Verify that the `--audit-log-path` argument is set as appropriate.

**Remediation:**
K3s server needs to be run with the following argument, `--kube-apiserver-arg='audit-log-path=/path/to/log/file'`.


#### 1.2.23
Ensure that the `--audit-log-maxage` argument is set to `30` or as appropriate (Scored)
<details>
<summary>Rationale</summary>
Retaining logs for at least 30 days ensures that you can go back in time and investigate or correlate any events. Set your audit log retention period to 30 days or as per your business requirements.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "audit-log-maxage"
```

Verify that the `--audit-log-maxage` argument is set to `30` or as appropriate.

**Remediation:**
K3s server needs to be run with the following argument, `--kube-apiserver-arg='audit-log-maxage=30'`.


#### 1.2.24
Ensure that the `--audit-log-maxbackup` argument is set to `10` or as appropriate (Scored)
<details>
<summary>Rationale</summary>
Kubernetes automatically rotates the log files. Retaining old log files ensures that you would have sufficient log data available for carrying out any investigation or correlation. For example, if you have set file size of 100 MB and the number of old log files to keep as 10, you would approximate have 1 GB of log data that you could potentially use for your analysis.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "audit-log-maxbackup"
```

Verify that the `--audit-log-maxbackup` argument is set to `10` or as appropriate.

**Remediation:**
K3s server needs to be run with the following argument, `--kube-apiserver-arg='audit-log-maxbackup=10'`.


#### 1.2.25
Ensure that the `--audit-log-maxsize` argument is set to `100` or as appropriate (Scored)
<details>
<summary>Rationale</summary>
Kubernetes automatically rotates the log files. Retaining old log files ensures that you would have sufficient log data available for carrying out any investigation or correlation. If you have set file size of 100 MB and the number of old log files to keep as 10, you would approximate have 1 GB of log data that you could potentially use for your analysis.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "audit-log-maxsize"
```

Verify that the `--audit-log-maxsize` argument is set to `100` or as appropriate.

**Remediation:**
K3s server needs to be run with the following argument, `--kube-apiserver-arg='audit-log-maxsize=100'`.


#### 1.2.26
Ensure that the `--request-timeout` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
Setting global request timeout allows extending the API server request timeout limit to a duration appropriate to the user's connection speed. By default, it is set to 60 seconds which might be problematic on slower connections making cluster resources inaccessible once the data volume for requests exceeds what can be transmitted in 60 seconds. But, setting this timeout limit to be too large can exhaust the API server resources making it prone to Denial-of-Service attack. Hence, it is recommended to set this limit as appropriate and change the default limit of 60 seconds only if needed.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "request-timeout"
```

Verify that the `--request-timeout` argument is either not set or set to an appropriate value.

**Remediation:**
By default, K3s does not set the `--request-timeout` argument. No manual remediation needed.


#### 1.2.27
Ensure that the `--service-account-lookup` argument is set to `true` (Scored)
<details>
<summary>Rationale</summary>
If `--service-account-lookup` is not enabled, the apiserver only verifies that the authentication token is valid, and does not validate that the service account token mentioned in the request is actually present in etcd. This allows using a service account token even after the corresponding service account is deleted. This is an example of time of check to time of use security issue.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "service-account-lookup"
```

Verify that if the `--service-account-lookup` argument exists it is set to `true`.

**Remediation:**
K3s server needs to be run with the following argument, `--kube-apiserver-arg='service-account-lookup=true'`.


#### 1.2.28
Ensure that the `--service-account-key-file` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
By default, if no `--service-account-key-file` is specified to the apiserver, it uses the private key from the TLS serving certificate to verify service account tokens. To ensure that the keys for service account tokens could be rotated as needed, a separate public/private key pair should be used for signing service account tokens. Hence, the public key should be specified to the apiserver with `--service-account-key-file`.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "service-account-key-file"
```

Verify that the `--service-account-key-file` argument exists and is set as appropriate.

**Remediation:**
By default, K3s sets the `--service-account-key-file` explicitly. No manual remediation needed.


#### 1.2.29
Ensure that the `--etcd-certfile` and `--etcd-keyfile` arguments are set as appropriate (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be protected by client authentication. This requires the API server to identify itself to the etcd server using a client certificate and key.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep -E 'etcd-certfile|etcd-keyfile'
```

Verify that the `--etcd-certfile` and `--etcd-keyfile` arguments exist and they are set as appropriate.

**Remediation:**
By default, K3s sets the `--etcd-certfile` and `--etcd-keyfile` arguments explicitly. No manual remediation needed.


#### 1.2.30
Ensure that the `--tls-cert-file` and `--tls-private-key-file` arguments are set as appropriate (Scored)
<details>
<summary>Rationale</summary>
API server communication contains sensitive parameters that should remain encrypted in transit. Configure the API server to serve only HTTPS traffic.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep -E 'tls-cert-file|tls-private-key-file'
```

Verify that the `--tls-cert-file` and `--tls-private-key-file` arguments exist and they are set as appropriate.

**Remediation:**
By default, K3s sets the `--tls-cert-file` and `--tls-private-key-file` arguments explicitly. No manual remediation needed.


#### 1.2.31
Ensure that the `--client-ca-file` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
API server communication contains sensitive parameters that should remain encrypted in transit. Configure the API server to serve only HTTPS traffic. If `--client-ca-file` argument is set, any request presenting a client certificate signed by one of the authorities in the `client-ca-file` is authenticated with an identity corresponding to the CommonName of the client certificate.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "client-ca-file"
```

Verify that the `--client-ca-file` argument exists and it is set as appropriate.

**Remediation:**
By default, K3s sets the `--client-ca-file` argument explicitly. No manual remediation needed.


#### 1.2.32
Ensure that the `--etcd-cafile` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be protected by client authentication. This requires the API server to identify itself to the etcd server using a SSL Certificate Authority file.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "etcd-cafile"
```

Verify that the `--etcd-cafile` argument exists and it is set as appropriate.

**Remediation:**
By default, K3s sets the `--etcd-cafile` argument explicitly. No manual remediation needed.


#### 1.2.33
Ensure that the `--encryption-provider-config` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be encrypted at rest to avoid any disclosures.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "encryption-provider-config"
```

Verify that the `--encryption-provider-config` argument is set to a EncryptionConfigfile. Additionally, ensure that the `EncryptionConfigfile` has all the desired resources covered especially any secrets.

**Remediation:**
K3s server needs to be ran with the follow, `--kube-apiserver-arg='encryption-provider-config=/path/to/encryption_config'`. This can be done by running k3s with the `--secrets-encryptiuon` argument which will configure the encryption provider.


#### 1.2.34
Ensure that encryption providers are appropriately configured (Scored)
<details>
<summary>Rationale</summary>
Where `etcd` encryption is used, it is important to ensure that the appropriate set of encryption providers is used. Currently, the `aescbc`, `kms` and `secretbox` are likely to be appropriate options.
</details>

**Result:** Pass

**Remediation:**
Follow the Kubernetes documentation and configure a `EncryptionConfig` file.
In this file, choose **aescbc**, **kms** or **secretbox** as the encryption provider.

**Audit:**
Run the below command on the master node.

```bash
grep aescbc /path/to/encryption-config.json
```

Run the below command on the master node.

Verify that aescbc is set as the encryption provider for all the desired resources.

**Remediation**
K3s server needs to be run with the following, `--secrets-encryption=true`, and verify that one of the allowed encryption providers is present.


#### 1.2.35
Ensure that the API Server only makes use of Strong Cryptographic Ciphers (Not Scored)

<details>
<summary>Rationale</summary>
TLS ciphers have had a number of known vulnerabilities and weaknesses, which can reduce the protection provided by them. By default Kubernetes supports a number of TLS cipher suites including some that have security concerns, weakening the protection provided.
</details>

**Result:** **Not Scored - Operator Dependent**

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "tls-cipher-suites"
```

Verify that the `--tls-cipher-suites` argument is set as outlined in the remediation procedure below.

**Remediation:**
By default, K3s explicitly doesn't set this flag. No manual remediation needed.


### 1.3 Controller Manager

#### 1.3.1
Ensure that the `--terminated-pod-gc-threshold` argument is set as appropriate (Not Scored)
<details>
<summary>Rationale</summary>
Garbage collection is important to ensure sufficient resource availability and avoiding degraded performance and availability. In the worst case, the system might crash or just be unusable for a long period of time. The current setting for garbage collection is 12,500 terminated pods which might be too high for your system to sustain. Based on your system resources and tests, choose an appropriate threshold value to activate garbage collection.
</details>

**Result:** **Not Scored - Operator Dependent**

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-controller-manager" | tail -n1 | grep "terminated-pod-gc-threshold
```

Verify that the `--terminated-pod-gc-threshold` argument is set as appropriate.

**Remediation:**
K3s server needs to be run with the following, `--kube-controller-manager-arg='terminated-pod-gc-threshold=10`.


#### 1.3.2
Ensure that the `--profiling` argument is set to false (Scored)
<details>
<summary>Rationale</summary>
Profiling allows for the identification of specific performance bottlenecks. It generates a significant amount of program data that could potentially be exploited to uncover system and program details. If you are not experiencing any bottlenecks and do not need the profiler for troubleshooting purposes, it is recommended to turn it off to reduce the potential attack surface.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-controller-manager" | tail -n1 | grep "profiling"
```

Verify that the `--profiling` argument is set to false.

**Remediation:**
By default, K3s sets the `--profiling` flag parameter to false. No manual remediation needed.


#### 1.3.3
Ensure that the `--use-service-account-credentials` argument is set to `true` (Scored)
<details>
<summary>Rationale</summary>
The controller manager creates a service account per controller in the `kube-system` namespace, generates a credential for it, and builds a dedicated API client with that service account credential for each controller loop to use. Setting the `--use-service-account-credentials` to `true` runs each control loop within the controller manager using a separate service account credential. When used in combination with RBAC, this ensures that the control loops run with the minimum permissions required to perform their intended tasks.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-controller-manager" | tail -n1 | grep "use-service-account-credentials"
```

Verify that the `--use-service-account-credentials` argument is set to true.

**Remediation:**
K3s server needs to be run with the following, `--kube-controller-manager-arg='use-service-account-credentials=true'`


#### 1.3.4
Ensure that the `--service-account-private-key-file` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
To ensure that keys for service account tokens can be rotated as needed, a separate public/private key pair should be used for signing service account tokens. The private key should be specified to the controller manager with `--service-account-private-key-file` as appropriate.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-controller-manager" | tail -n1 | grep "service-account-private-key-file"
```

Verify that the `--service-account-private-key-file` argument is set as appropriate.

**Remediation:**
By default, K3s sets the `--service-account-private-key-file` argument with the service account key file. No manual remediation needed.


#### 1.3.5
Ensure that the `--root-ca-file` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
Processes running within pods that need to contact the API server must verify the API server's serving certificate. Failing to do so could be a subject to man-in-the-middle attacks.

Providing the root certificate for the API server's serving certificate to the controller manager with the `--root-ca-file` argument allows the controller manager to inject the trusted bundle into pods so that they can verify TLS connections to the API server.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-controller-manager" | tail -n1 | grep "root-ca-file"
```

Verify that the `--root-ca-file` argument exists and is set to a certificate bundle file containing the root certificate for the API server's serving certificate

**Remediation:**
By default, K3s sets the `--root-ca-file` argument with the root ca file. No manual remediation needed.


#### 1.3.6
Ensure that the `RotateKubeletServerCertificate` argument is set to `true` (Scored)
<details>
<summary>Rationale</summary>
`RotateKubeletServerCertificate` causes the kubelet to both request a serving certificate after bootstrapping its client credentials and rotate the certificate as its existing credentials expire. This automated periodic rotation ensures that there are no downtimes due to expired certificates and thus addressing availability in the CIA security triad.

Note: This recommendation only applies if you let kubelets get their certificates from the API server. In case your kubelet certificates come from an outside authority/tool (e.g. Vault) then you need to take care of rotation yourself.
</details>

**Result:** Not Applicable

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-controller-manager" | tail -n1 | grep "RotateKubeletServerCertificate"
```

Verify that RotateKubeletServerCertificateargument exists and is set to true.

**Remediation:**
By default, K3s implements its own logic for certificate generation and rotation.


#### 1.3.7
Ensure that the `--bind-address` argument is set to `127.0.0.1` (Scored)
<details>
<summary>Rationale</summary>
The Controller Manager API service which runs on port 10252/TCP by default is used for health and metrics information and is available without authentication or encryption. As such it should only be bound to a localhost interface, to minimize the cluster's attack surface.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-controller-manager" | tail -n1 | grep "bind-address"
```

Verify that the `--bind-address` argument is set to 127.0.0.1.

**Remediation:**
By default, K3s sets the `--bind-address` argument to `127.0.0.1`. No manual remediation needed.


### 1.4 Scheduler
This section contains recommendations relating to Scheduler configuration flags


#### 1.4.1
Ensure that the `--profiling` argument is set to `false` (Scored)
<details>
<summary>Rationale</summary>
Profiling allows for the identification of specific performance bottlenecks. It generates a significant amount of program data that could potentially be exploited to uncover system and program details. If you are not experiencing any bottlenecks and do not need the profiler for troubleshooting purposes, it is recommended to turn it off to reduce the potential attack surface.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-scheduler" | tail -n1 | grep "profiling"
```

Verify that the `--profiling` argument is set to false.

**Remediation:**
By default, K3s sets the `--profiling` flag parameter to false. No manual remediation needed.


#### 1.4.2
Ensure that the `--bind-address` argument is set to `127.0.0.1` (Scored)
<details>
<summary>Rationale</summary>

The Scheduler API service which runs on port 10251/TCP by default is used for health and metrics information and is available without authentication or encryption. As such it should only be bound to a localhost interface, to minimize the cluster's attack surface.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-scheduler" | tail -n1 | grep "bind-address"
```

Verify that the `--bind-address` argument is set to 127.0.0.1.

**Remediation:**
By default, K3s sets the `--bind-address` argument to `127.0.0.1`. No manual remediation needed.


## 2 Etcd Node Configuration
This section covers recommendations for etcd configuration.

#### 2.1
Ensure that the `cert-file` and `key-file` fields are set as appropriate (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be encrypted in transit.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
grep -E 'cert-file|key-file' /var/lib/rancher/k3s/server/db/etcd/config
```

Verify that the	`cert-file` and the `key-file` fields are set as appropriate.

**Remediation:**
By default, K3s uses a config file for etcd that can be found at `/var/lib/rancher/k3s/server/db/etcd/config`. Server and peer cert and key files are specified. No manual remediation needed.


#### 2.2
Ensure that the `client-cert-auth` field is set to `true` (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should not be available to unauthenticated clients. You should enable the client authentication via valid certificates to secure the access to the etcd service.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
grep 'client-cert-auth' /var/lib/rancher/k3s/server/db/etcd/config
```

Verify that the `client-cert-auth` field is set to true.

**Remediation:**
By default, K3s uses a config file for etcd that can be found at `/var/lib/rancher/k3s/server/db/etcd/config`. `client-cert-auth` is set to true. No manual remediation needed.


#### 2.3
Ensure that the `auto-tls` field is not set to `true` (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should not be available to unauthenticated clients. You should enable the client authentication via valid certificates to secure the access to the etcd service.
</details>

**Result:** Pass

**Remediation:**
By default, K3s starts Etcd without this flag. It is set to `false` by default.


#### 2.4
Ensure that the `peer-cert-file` and `peer-key-file` fields are set as appropriate (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be encrypted in transit and also amongst peers in the etcd clusters.
</details>

**Result:** Pass

**Remediation:**
By default, K3s starts Etcd with a config file found here, `/var/lib/rancher/k3s/server/db/etcd/config`. The config file contains `peer-transport-security:` which has fields that have the peer cert and peer key files.


#### 2.5
Ensure that the `client-cert-auth` field is set to `true` (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be accessible only by authenticated etcd peers in the etcd cluster.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
grep 'client-cert-auth' /var/lib/rancher/k3s/server/db/etcd/config
```

Verify that the `client-cert-auth` field in the peer section is set to true.

**Remediation:**
By default, K3s uses a config file for etcd that can be found at `/var/lib/rancher/k3s/server/db/etcd/config`. Within the file, the `client-cert-auth` field is set. No manual remediation needed.


#### 2.6
Ensure that the `peer-auto-tls` field is not set to `true` (Scored)
<details>
<summary>Rationale</summary>
etcd is a highly-available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be accessible only by authenticated etcd peers in the etcd cluster. Hence, do not use self- signed certificates for authentication.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
grep 'peer-auto-tls' /var/lib/rancher/k3s/server/db/etcd/config
```

Verify that if the `peer-auto-tls` field does not exist.

**Remediation:**
By default, K3s uses a config file for etcd that can be found at `/var/lib/rancher/k3s/server/db/etcd/config`. Within the file, it does not contain the `peer-auto-tls` field. No manual remediation needed.


#### 2.7
Ensure that a unique Certificate Authority is used for etcd (Not Scored)
<details>
<summary>Rationale</summary>
etcd is a highly available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. Its access should be restricted to specifically designated clients and peers only.

Authentication to etcd is based on whether the certificate presented was issued by a trusted certificate authority. There is no checking of certificate attributes such as common name or subject alternative name. As such, if any attackers were able to gain access to any certificate issued by the trusted certificate authority, they would be able to gain full access to the etcd database.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
# To find the ca file used by etcd:
grep 'trusted-ca-file' /var/lib/rancher/k3s/server/db/etcd/config
# To find the kube-apiserver process:
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1
```

Verify that the file referenced by the `client-ca-file` flag in the apiserver process is different from the file referenced by the `trusted-ca-file` parameter in the etcd configuration file.

**Remediation:**
By default, K3s uses a config file for etcd that can be found at `/var/lib/rancher/k3s/server/db/etcd/config` and the `trusted-ca-file` parameters in it are set to unique values specific to etcd. No manual remediation needed.



## 3 Control Plane Configuration


### 3.1 Authentication and Authorization


#### 3.1.1
Client certificate authentication should not be used for users (Not Scored)
<details>
<summary>Rationale</summary>
With any authentication mechanism the ability to revoke credentials if they are compromised or no longer required, is a key control. Kubernetes client certificate authentication does not allow for this due to a lack of support for certificate revocation.
</details>

**Result:** Not Scored - Operator Dependent

**Audit:**
Review user access to the cluster and ensure that users are not making use of Kubernetes client certificate authentication.

**Remediation:**
Alternative mechanisms provided by Kubernetes such as the use of OIDC should be implemented in place of client certificates.

### 3.2 Logging


#### 3.2.1
Ensure that a minimal audit policy is created (Scored)
<details>
<summary>Rationale</summary>
Logging is an important detective control for all systems, to detect potential unauthorized access.
</details>

**Result:** Does not pass. See the [Hardening Guide](../hardening_guide/) for details.

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "audit-policy-file"
```

Verify that the `--audit-policy-file` is set. Review the contents of the file specified and ensure that it contains a valid audit policy.

**Remediation:**
Create an audit policy file for your cluster and pass it to k3s. e.g. `--kube-apiserver-arg='audit-log-path=/var/lib/rancher/k3s/server/logs/audit-log'`


#### 3.2.2
Ensure that the audit policy covers key security concerns (Not Scored)
<details>
<summary>Rationale</summary>
Security audit logs should cover access and modification of key resources in the cluster, to enable them to form an effective part of a security environment.
</details>

**Result:** Not Scored - Operator Dependent

**Remediation:**


## 4 Worker Node Security Configuration


### 4.1 Worker Node Configuration Files


#### 4.1.1
Ensure that the kubelet service file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The `kubelet` service file controls various parameters that set the behavior of the kubelet service in the worker node. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Not Applicable

**Remediation:**
K3s doesn’t launch the kubelet as a service. It is launched and managed by the K3s supervisor process. All configuration is passed to it as command line arguments at run time.


#### 4.1.2
Ensure that the kubelet service file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The `kubelet` service file controls various parameters that set the behavior of the kubelet service in the worker node. You should set its file ownership to maintain the integrity of the file. The file should be owned by `root:root`.
</details>

**Result:** Not Applicable

**Remediation:**
K3s doesn’t launch the kubelet as a service. It is launched and managed by the K3s supervisor process. All configuration is passed to it as command line arguments at run time.


#### 4.1.3
Ensure that the proxy kubeconfig file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The `kube-proxy` kubeconfig file controls various parameters of the `kube-proxy` service in the worker node. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.

It is possible to run `kube-proxy` with the kubeconfig parameters configured as a Kubernetes ConfigMap instead of a file. In this case, there is no proxy kubeconfig file.
</details>

**Result:** Not Applicable

**Audit:**
Run the below command on the worker node.

```bash
stat -c %a /var/lib/rancher/k3s/agent/kubeproxy.kubeconfig
644
```

Verify that if a file is specified and it exists, the permissions are 644 or more restrictive.

**Remediation:**
K3s runs `kube-proxy` in process and does not use a config file.


#### 4.1.4
Ensure that the proxy kubeconfig file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The kubeconfig file for `kube-proxy` controls various parameters for the `kube-proxy` service in the worker node. You should set its file ownership to maintain the integrity of the file. The file should be owned by `root:root`.
</details>

**Result:** Not Applicable

**Audit:**
Run the below command on the master node.

```bash
stat -c %U:%G /var/lib/rancher/k3s/agent/kubeproxy.kubeconfig
root:root
```

Verify that if a file is specified and it exists, the permissions are 644 or more restrictive.

**Remediation:**
K3s runs `kube-proxy` in process and does not use a config file.


#### 4.1.5
Ensure that the kubelet.conf file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The `kubelet.conf` file is the kubeconfig file for the node, and controls various parameters that set the behavior and identity of the worker node. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Pass

**Audit:**
Run the below command on the worker node.

```bash
stat -c %a /var/lib/rancher/k3s/agent/kubelet.kubeconfig
644
```

**Remediation:**
By default, K3s creates `kubelet.kubeconfig` with `644` permissions. No manual remediation needed.

#### 4.1.6
Ensure that the kubelet.conf file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The `kubelet.conf` file is the kubeconfig file for the node, and controls various parameters that set the behavior and identity of the worker node. You should set its file ownership to maintain the integrity of the file. The file should be owned by `root:root`.
</details>

**Result:** Not Applicable

**Audit:**
Run the below command on the master node.

```bash
stat -c %U:%G /var/lib/rancher/k3s/agent/kubelet.kubeconfig
root:root
```

**Remediation:**
By default, K3s creates `kubelet.kubeconfig` with `root:root` ownership. No manual remediation needed.


#### 4.1.7
Ensure that the certificate authorities file permissions are set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The certificate authorities file controls the authorities used to validate API requests. You should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
stat -c %a /var/lib/rancher/k3s/server/tls/server-ca.crt
644
```

Verify that the permissions are 644.

**Remediation:**
By default, K3s creates `/var/lib/rancher/k3s/server/tls/server-ca.crt` with `644` permissions.


#### 4.1.8
Ensure that the client certificate authorities file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The certificate authorities file controls the authorities used to validate API requests. You should set its file ownership to maintain the integrity of the file. The file should be owned by `root:root`.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
stat -c %U:%G /var/lib/rancher/k3s/server/tls/client-ca.crt
root:root
```

**Remediation:**
By default, K3s creates `/var/lib/rancher/k3s/server/tls/client-ca.crt` with `root:root` ownership.


#### 4.1.9
Ensure that the kubelet configuration file has permissions set to `644` or more restrictive (Scored)
<details>
<summary>Rationale</summary>
The kubelet reads various parameters, including security settings, from a config file specified by the `--config` argument. If this file is specified you should restrict its file permissions to maintain the integrity of the file. The file should be writable by only the administrators on the system.
</details>

**Result:** Not Applicable

**Remediation:**
K3s doesn’t require or maintain a configuration file for the kubelet process. All configuration is passed to it as command line arguments at run time.


#### 4.1.10
Ensure that the kubelet configuration file ownership is set to `root:root` (Scored)
<details>
<summary>Rationale</summary>
The kubelet reads various parameters, including security settings, from a config file specified by the `--config` argument. If this file is specified you should restrict its file permissions to maintain the integrity of the file. The file should be owned by `root:root`.
</details>

**Result:** Not Applicable

**Remediation:**
K3s doesn’t require or maintain a configuration file for the kubelet process. All configuration is passed to it as command line arguments at run time.


### 4.2 Kubelet
This section contains recommendations for kubelet configuration.


#### 4.2.1
Ensure that the `--anonymous-auth` argument is set to false (Scored)
<details>
<summary>Rationale</summary>
When enabled, requests that are not rejected by other configured authentication methods are treated as anonymous requests. These requests are then served by the Kubelet server. You should rely on authentication to authorize access and disallow anonymous requests.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "anonymous-auth"
```

Verify that the value for `--anonymous-auth` is false.

**Remediation:**
By default, K3s starts kubelet with `--anonymous-auth` set to false. No manual remediation needed.

#### 4.2.2
Ensure that the `--authorization-mode` argument is not set to `AlwaysAllow` (Scored)
<details>
<summary>Rationale</summary>
Kubelets, by default, allow all authenticated requests (even anonymous ones) without needing explicit authorization checks from the apiserver. You should restrict this behavior and only allow explicitly authorized requests.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "authorization-mode"
```

Verify that `AlwaysAllow` is not present.

**Remediation:**
K3s starts kubelet with `Webhook` as the value for the `--authorization-mode` argument. No manual remediation needed.


#### 4.2.3
Ensure that the `--client-ca-file` argument is set as appropriate (Scored)
<details>
<summary>Rationale</summary>
The connections from the apiserver to the kubelet are used for fetching logs for pods, attaching (through kubectl) to running pods, and using the kubelet’s port-forwarding functionality. These connections terminate at the kubelet’s HTTPS endpoint. By default, the apiserver does not verify the kubelet’s serving certificate, which makes the connection subject to man-in-the-middle attacks, and unsafe to run over untrusted and/or public networks. Enabling Kubelet certificate authentication ensures that the apiserver could authenticate the Kubelet before submitting any requests.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kube-apiserver" | tail -n1 | grep "client-ca-file"
```

Verify that the `--client-ca-file` argument has a ca file associated.

**Remediation:**
By default, K3s starts the kubelet process with the `--client-ca-file`. No manual remediation needed.


#### 4.2.4
Ensure that the `--read-only-port` argument is set to `0` (Scored)
<details>
<summary>Rationale</summary>
The Kubelet process provides a read-only API in addition to the main Kubelet API. Unauthenticated access is provided to this read-only API which could possibly retrieve potentially sensitive information about the cluster.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kubelet" | tail -n1 | grep "read-only-port"
```
Verify that the `--read-only-port` argument is set to 0.

**Remediation:**
By default, K3s starts the kubelet process with the `--read-only-port` argument set to `0`.


#### 4.2.5
Ensure that the `--streaming-connection-idle-timeout` argument is not set to `0` (Scored)
<details>
<summary>Rationale</summary>
Setting idle timeouts ensures that you are protected against Denial-of-Service attacks, inactive connections and running out of ephemeral ports.

**Note:** By default, `--streaming-connection-idle-timeout` is set to 4 hours which might be too high for your environment. Setting this as appropriate would additionally ensure that such streaming connections are timed out after serving legitimate use cases.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kubelet" | tail -n1 | grep "streaming-connection-idle-timeout"
```

Verify that there's nothing returned.

**Remediation:**
By default, K3s does not set `--streaming-connection-idle-timeout` when starting kubelet.


#### 4.2.6
Ensure that the `--protect-kernel-defaults` argument is set to `true` (Scored)
<details>
<summary>Rationale</summary>
Kernel parameters are usually tuned and hardened by the system administrators before putting the systems into production. These parameters protect the kernel and the system. Your kubelet kernel defaults that rely on such parameters should be appropriately set to match the desired secured system state. Ignoring this could potentially lead to running pods with undesired kernel behavior.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kubelet" | tail -n1 | grep "protect-kernel-defaults"
```

**Remediation:**
K3s server needs to be started with the following, `--protect-kernel-defaults=true`.


#### 4.2.7
Ensure that the `--make-iptables-util-chains` argument is set to `true` (Scored)
<details>
<summary>Rationale</summary>
Kubelets can automatically manage the required changes to iptables based on how you choose your networking options for the pods. It is recommended to let kubelets manage the changes to iptables. This ensures that the iptables configuration remains in sync with pods networking configuration. Manually configuring iptables with dynamic pod network configuration changes might hamper the communication between pods/containers and to the outside world. You might have iptables rules too restrictive or too open.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kubelet" | tail -n1 | grep "make-iptables-util-chains"
```

Verify there are no results returned.

**Remediation:**
K3s server needs to be run with the following, `--kube-apiserver-arg='make-iptables-util-chains=true'`.


#### 4.2.8
Ensure that the `--hostname-override` argument is not set (Not Scored)
<details>
<summary>Rationale</summary>
Overriding hostnames could potentially break TLS setup between the kubelet and the apiserver. Additionally, with overridden hostnames, it becomes increasingly difficult to associate logs with a particular node and process them for security analytics. Hence, you should setup your kubelet nodes with resolvable FQDNs and avoid overriding the hostnames with IPs.
</details>

**Result:** Not Applicable

**Remediation:**
K3s does set this parameter for each host, but K3s also manages all certificates in the cluster. It ensures the hostname-override is included as a subject alternative name (SAN) in the kubelet's certificate.


#### 4.2.9
Ensure that the `--event-qps` argument is set to 0 or a level which ensures appropriate event capture (Not Scored)
<details>
<summary>Rationale</summary>
It is important to capture all events and not restrict event creation. Events are an important source of security information and analytics that ensure that your environment is consistently monitored using the event data.
</details>

**Result:** Not Scored - Operator Dependent

**Remediation:**
See CIS Benchmark guide for further details on configuring this.

#### 4.2.10
Ensure that the `--tls-cert-file` and `--tls-private-key-file` arguments are set as appropriate (Scored)
<details>
<summary>Rationale</summary>
Kubelet communication contains sensitive parameters that should remain encrypted in transit. Configure the Kubelets to serve only HTTPS traffic.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
journalctl -u k3s | grep "Running kubelet" | tail -n1 | grep -E 'tls-cert-file|tls-private-key-file'
```

Verify the `--tls-cert-file` and `--tls-private-key-file` arguments are present and set appropriately.

**Remediation:**
By default, K3s sets the `--tls-cert-file` and `--tls-private-key-file` arguments when executing the kubelet process.


#### 4.2.11
Ensure that the `--rotate-certificates` argument is not set to `false` (Scored)
<details>
<summary>Rationale</summary>

The `--rotate-certificates` setting causes the kubelet to rotate its client certificates by creating new CSRs as its existing credentials expire. This automated periodic rotation ensures that there is no downtime due to expired certificates and thus addressing availability in the CIA security triad.

**Note:** This recommendation only applies if you let kubelets get their certificates from the API server. In case your kubelet certificates come from an outside authority/tool (e.g. Vault) then you need to take care of rotation yourself.

**Note:**This feature also requires the `RotateKubeletClientCertificate` feature gate to be enabled (which is the default since Kubernetes v1.7)
</details>

**Result:** Not Applicable

**Remediation:**
By default, K3s implements its own logic for certificate generation and rotation.


#### 4.2.12
Ensure that the `RotateKubeletServerCertificate` argument is set to `true` (Scored)
<details>
<summary>Rationale</summary>
`RotateKubeletServerCertificate` causes the kubelet to both request a serving certificate after bootstrapping its client credentials and rotate the certificate as its existing credentials expire. This automated periodic rotation ensures that there are no downtimes due to expired certificates and thus addressing availability in the CIA security triad.

Note: This recommendation only applies if you let kubelets get their certificates from the API server. In case your kubelet certificates come from an outside authority/tool (e.g. Vault) then you need to take care of rotation yourself.
</details>

**Result:** Not Applicable

**Remediation:**
By default, K3s implements its own logic for certificate generation and rotation.


#### 4.2.13
Ensure that the Kubelet only makes use of Strong Cryptographic Ciphers (Not Scored)
<details>
<summary>Rationale</summary>
TLS ciphers have had a number of known vulnerabilities and weaknesses, which can reduce the protection provided by them. By default Kubernetes supports a number of TLS ciphersuites including some that have security concerns, weakening the protection provided.
</details>

**Result:** Not Scored - Operator Dependent

**Remediation:**
Configuration of the parameter is dependent on your use case. Please see the CIS Kubernetes Benchmark for suggestions on configuring this for your use-case.


## 5 Kubernetes Policies


### 5.1 RBAC and Service Accounts


#### 5.1.1
Ensure that the cluster-admin role is only used where required (Not Scored)
<details>
<summary>Rationale</summary>
Kubernetes provides a set of default roles where RBAC is used. Some of these roles such as `cluster-admin` provide wide-ranging privileges which should only be applied where absolutely necessary. Roles such as `cluster-admin` allow super-user access to perform any action on any resource. When used in a `ClusterRoleBinding`, it gives full control over every resource in the cluster and in all namespaces. When used in a `RoleBinding`, it gives full control over every resource in the rolebinding's namespace, including the namespace itself.
</details>

**Result:** Pass

**Remediation:**
K3s does not make inappropriate use of the cluster-admin role. Operators must audit their workloads of additional usage. See the CIS Benchmark guide for more details.

#### 5.1.2
Minimize access to secrets (Not Scored)
<details>
<summary>Rationale</summary>
Inappropriate access to secrets stored within the Kubernetes cluster can allow for an attacker to gain additional access to the Kubernetes cluster or external resources whose credentials are stored as secrets.
</details>

**Result:** Not Scored - Operator Dependent

**Remediation:**
K3s limits its use of secrets for the system components appropriately, but operators must audit the use of secrets by their workloads. See the CIS Benchmark guide for more details.

#### 5.1.3
Minimize wildcard use in Roles and ClusterRoles (Not Scored)
<details>
<summary>Rationale</summary>
The principle of least privilege recommends that users are provided only the access required for their role and nothing more. The use of wildcard rights grants is likely to provide excessive rights to the Kubernetes API.
</details>

**Result:** Not Scored - Operator Dependent

**Audit:**
Run the below command on the master node.

```bash
# Retrieve the roles defined across each namespaces in the cluster and review for wildcards
kubectl get roles --all-namespaces -o yaml

# Retrieve the cluster roles defined in the	cluster	and	review for wildcards
kubectl get clusterroles -o yaml
```

Verify that there are not wildcards in use.

**Remediation:**
Operators should review their workloads for proper role usage. See the CIS Benchmark guide for more details.

#### 5.1.4
Minimize access to create pods (Not Scored)
<details>
<summary>Rationale</summary>
The ability to create pods in a cluster opens up possibilities for privilege escalation and should be restricted, where possible.
</details>

**Result:** Not Scored - Operator Dependent

**Remediation:**
Operators should review who has access to create pods in their cluster. See the CIS Benchmark guide for more details.

#### 5.1.5
Ensure that default service accounts are not actively used. (Scored)
<details>
<summary>Rationale</summary>
Kubernetes provides a default service account which is used by cluster workloads where no specific service account is assigned to the pod.

Where access to the Kubernetes API from a pod is required, a specific service account should be created for that pod, and rights granted to that service account.

The default service account should be configured such that it does not provide a service account token and does not have any explicit rights assignments.
</details>

**Result:** Fail. Currently requires operator intervention See the [Hardening Guide](hardening_guide.md) for details.

**Audit:**
For	each namespace in the cluster, review the rights assigned to the default service account and ensure that it has no roles or cluster roles bound to it apart from the defaults. Additionally ensure that the automountServiceAccountToken: false setting is in place for each default service account.

**Remediation:**
Create explicit service accounts wherever a Kubernetes workload requires specific access
to the Kubernetes API server.
Modify the configuration of each default service account to include this value

``` bash
automountServiceAccountToken: false
```


#### 5.1.6
Ensure that Service Account Tokens are only mounted where necessary (Not Scored)
<details>
<summary>Rationale</summary>
Mounting service account tokens inside pods can provide an avenue for privilege escalation attacks where an attacker is able to compromise a single pod in the cluster.

Avoiding mounting these tokens removes this attack avenue.
</details>

**Result:** Not Scored - Operator Dependent

**Remediation:**
The pods launched by K3s are part of the control plane and generally need access to communicate with the API server, thus this control does not apply to them. Operators should review their workloads and take steps to modify the definition of pods and service accounts which do not need to mount service account tokens to disable it.

### 5.2 Pod Security Policies


#### 5.2.1
Minimize the admission of containers wishing to share the host process ID namespace (Scored)
<details>
<summary>Rationale</summary>
Privileged containers have access to all Linux Kernel capabilities and devices. A container running with full privileges can do almost everything that the host can do. This flag exists to allow special use-cases, like manipulating the network stack and accessing devices.

There should be at least one PodSecurityPolicy (PSP) defined which does not permit privileged containers.

If you need to run privileged containers, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
kubectl describe psp <psp_name> | grep MustRunAsNonRoot
```

Verify that the result is `Rule:  MustRunAsNonRoot`.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets the `Rule` value to `MustRunAsNonRoot`. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.2
Minimize the admission of containers wishing to share the host process ID namespace (Scored)
<details>
<summary>Rationale</summary>
A container running in the host's PID namespace can inspect processes running outside the container. If the container also has access to ptrace capabilities this can be used to escalate privileges outside of the container.

There should be at least one PodSecurityPolicy (PSP) defined which does not permit containers to share the host PID namespace.

If you need to run containers which require hostPID, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp -o json | jq .items[] | jq -r 'select((.spec.hostPID == null) or (.spec.hostPID == false))' | jq .metadata.name | wc -l | xargs -I {} echo '--count={}'
```

Verify that the returned count is 1.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets the `hostPID` value to false explicitly for the PSP it creates. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.3
Minimize the admission of containers wishing to share the host IPC namespace (Scored)
<details>
<summary>Rationale</summary>

A container running in the host's IPC namespace can use IPC to interact with processes outside the container.

There should be at least one PodSecurityPolicy (PSP) defined which does not permit containers to share the host IPC namespace.

If you have a requirement to containers which require hostIPC, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp -o json | jq .items[] | jq -r 'select((.spec.hostIPC == null) or (.spec.hostIPC == false))' | jq .metadata.name | wc -l | xargs -I {} echo '--count={}'
```

Verify that the returned count is 1.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets the `HostIPC` value to false explicitly for the PSP it creates. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.4
Minimize the admission of containers wishing to share the host network namespace (Scored)
<details>
<summary>Rationale</summary>
A container running in the host's network namespace could access the local loopback device, and could access network traffic to and from other pods.

There should be at least one PodSecurityPolicy (PSP) defined which does not permit containers to share the host network namespace.

If you have need to run containers which require hostNetwork, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp -o json | jq .items[] | jq -r 'select((.spec.hostNetwork == null) or (.spec.hostNetwork == false))' | jq .metadata.name | wc -l | xargs -I {} echo '--count={}'
```

Verify that the returned count is 1.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets the `HostNetwork` value to false explicitly for the PSP it creates. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.5
Minimize the admission of containers with `allowPrivilegeEscalation` (Scored)
<details>
<summary>Rationale</summary>
A container running with the `allowPrivilegeEscalation` flag set to true may have processes that can gain more privileges than their parent.

There should be at least one PodSecurityPolicy (PSP) defined which does not permit containers to allow privilege escalation. The option exists (and is defaulted to true) to permit setuid binaries to run.

If you have need to run containers which use setuid binaries or require privilege escalation, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp -o json | jq .items[] | jq -r 'select((.spec.allowPrivilegeEscalation == null) or (.spec.allowPrivilegeEscalation == false))' | jq .metadata.name | wc -l | xargs -I {} echo '--count={}'
```

Verify that the returned count is 1.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets the `allowPrivilegeEscalation` value to false explicitly for the PSP it creates. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.6
Minimize the admission of root containers (Not Scored)
<details>
<summary>Rationale</summary>
Containers may run as any Linux user. Containers which run as the root user, whilst constrained by Container Runtime security features still have an escalated likelihood of container breakout.

Ideally, all containers should run as a defined non-UID 0 user.

There should be at least one PodSecurityPolicy (PSP) defined which does not permit root users in a container.

If you need to run root containers, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Not Scored

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp -o json | jq .items[] | jq -r 'select((.spec.allowPrivilegeEscalation == null) or (.spec.allowPrivilegeEscalation == false))' | jq .metadata.name | wc -l | xargs -I {} echo '--count={}'
```

Verify that the returned count is 1.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets the `runAsUser.Rule` value to `MustRunAsNonRoot`. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.7
Minimize the admission of containers with the NET_RAW capability (Not Scored)
<details>
<summary>Rationale</summary>
Containers run with a default set of capabilities as assigned by the Container Runtime. By default this can include potentially dangerous capabilities. With Docker as the container runtime the NET_RAW capability is enabled which may be misused by malicious containers.

Ideally, all containers should drop this capability.

There should be at least one PodSecurityPolicy (PSP) defined which prevents containers with the NET_RAW capability from launching.

If you need to run containers with this capability, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Not Scored

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp <psp_name> -o json | jq .spec.requiredDropCapabilities[]
```

Verify the value is `"ALL"`.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets `.spec.requiredDropCapabilities[]` to a value of `All`. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.8
Minimize the admission of containers with added capabilities (Not Scored)
<details>
<summary>Rationale</summary>
Containers run with a default set of capabilities as assigned by the Container Runtime. Capabilities outside this set can be added to containers which could expose them to risks of container breakout attacks.

There should be at least one PodSecurityPolicy (PSP) defined which prevents containers with capabilities beyond the default set from launching.

If you need to run containers with additional capabilities, this should be defined in a separate PSP and you should carefully check RBAC controls to ensure that only limited service accounts and users are given permission to access that PSP.
</details>

**Result:** Not Scored

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp
```

Verify that there are no PSPs present which have `allowedCapabilities` set to anything other than an empty array.

**Remediation:**
An operator should apply a PodSecurityPolicy that sets `allowedCapabilities` to anything other than an empty array. An example of this can be found in the [Hardening Guide](../hardening_guide/).


#### 5.2.9
Minimize the admission of containers with capabilities assigned (Not Scored)
<details>
<summary>Rationale</summary>
Containers run with a default set of capabilities as assigned by the Container Runtime. Capabilities are parts of the rights generally granted on a Linux system to the root user.

In many cases applications running in containers do not require any capabilities to operate, so from the perspective of the principle of least privilege use of capabilities should be minimized.
</details>

**Result:** Not Scored

**Audit:**
Run the below command on the master node.

```bash
kubectl get psp
```

**Remediation:**
An operator should apply a PodSecurityPolicy that sets `requiredDropCapabilities` to `ALL`. An example of this can be found in the [Hardening Guide](../hardening_guide/).


### 5.3 Network Policies and CNI


#### 5.3.1
Ensure that the CNI in use supports Network Policies (Not Scored)
<details>
<summary>Rationale</summary>
Kubernetes network policies are enforced by the CNI plugin in use. As such it is important to ensure that the CNI plugin supports both Ingress and Egress network policies.
</details>

**Result:** Pass

**Audit:**
Review the documentation of CNI plugin in use by the cluster, and confirm that it supports Ingress and Egress network policies.

**Remediation:**
By default, K3s use Canal (Calico and Flannel) and fully supports network policies.


#### 5.3.2
Ensure that all Namespaces have Network Policies defined (Scored)
<details>
<summary>Rationale</summary>
Running different applications on the same Kubernetes cluster creates a risk of one compromised application attacking a neighboring application. Network segmentation is important to ensure that containers can communicate only with those they are supposed to. A network policy is a specification of how selections of pods are allowed to communicate with each other and other network endpoints.

Network Policies are namespace scoped. When a network policy is introduced to a given namespace, all traffic not allowed by the policy is denied. However, if there are no network policies in a namespace all traffic will be allowed into and out of the pods in that namespace.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
for i in kube-system kube-public default; do
    kubectl get networkpolicies -n $i;
done
```

Verify that there are network policies applied to each of the namespaces.

**Remediation:**
An operator should apply NetworkPolcyies that prevent unneeded traffic from traversing networks unnecessarily. An example of applying a NetworkPolcy can be found in the [Hardening Guide](../hardening_guide/).

### 5.4 Secrets Management


#### 5.4.1
Prefer using secrets as files over secrets as environment variables (Not Scored)
<details>
<summary>Rationale</summary>
It is reasonably common for application code to log out its environment (particularly in the event of an error). This will include any secret values passed in as environment variables, so secrets can easily be exposed to any user or entity who has access to the logs.
</details>

**Result:** Not Scored

**Audit:**
Run the following command to find references to objects which use environment variables defined from secrets.

```bash
kubectl get all -o jsonpath='{range .items[?(@..secretKeyRef)]} {.kind} {.metadata.name} {"\n"}{end}' -A
```

**Remediation:**
If possible, rewrite application code to read secrets from mounted secret files, rather than from environment variables.


#### 5.4.2
Consider external secret storage (Not Scored)
<details>
<summary>Rationale</summary>
Kubernetes supports secrets as first-class objects, but care needs to be taken to ensure that access to secrets is carefully limited. Using an external secrets provider can ease the management of access to secrets, especially where secrets are used across both Kubernetes and non-Kubernetes environments.
</details>

**Result:** Not Scored

**Audit:**
Review your secrets management implementation.

**Remediation:**
Refer to the secrets management options offered by your cloud provider or a third-party secrets management solution.


### 5.5 Extensible Admission Control


#### 5.5.1
Configure Image Provenance using ImagePolicyWebhook admission controller (Not Scored)
<details>
<summary>Rationale</summary>
Kubernetes supports plugging in provenance rules to accept or reject the images in your deployments. You could configure such rules to ensure that only approved images are deployed in the cluster.
</details>

**Result:** Not Scored

**Audit:**
Review the pod definitions in your cluster and verify that image _provenance_ is configured as appropriate.

**Remediation:**
Follow the Kubernetes documentation and setup image provenance.


### 5.6 Omitted
The v1.5.1 Benchmark skips 5.6 and goes from 5.5 to 5.7. We are including it here merely for explanation.


### 5.7 General Policies
These policies relate to general cluster management topics, like namespace best practices and policies applied to pod objects in the cluster.


#### 5.7.1
Create administrative boundaries between resources using namespaces (Not Scored)
<details>
<summary>Rationale</summary>
Limiting the scope of user permissions can reduce the impact of mistakes or malicious activities. A Kubernetes namespace allows you to partition created resources into logically named groups. Resources created in one namespace can be hidden from other namespaces. By default, each resource created by a user in Kubernetes cluster runs in a default namespace, called default. You can create additional namespaces and attach resources and users to them. You can use Kubernetes Authorization plugins to create policies that segregate access to namespace resources between different users.
</details>

**Result:** Not Scored

**Audit:**
Run the below command and review the namespaces created in the cluster.

```bash
kubectl get namespaces
```

Ensure that these namespaces are the ones you need and are adequately administered as per your requirements.

**Remediation:**
Follow the documentation and create namespaces for objects in your deployment as you need them.


#### 5.7.2
Ensure that the seccomp profile is set to `docker/default` in your pod definitions (Not Scored)
<details>
<summary>Rationale</summary>
Seccomp (secure computing mode) is used to restrict the set of system calls applications can make, allowing cluster administrators greater control over the security of workloads running in the cluster. Kubernetes disables seccomp profiles by default for historical reasons. You should enable it to ensure that the workloads have restricted actions available within the container.
</details>

**Result:** Not Scored

**Audit:**
Review the pod definitions in your cluster. It should create a line as below:

```yaml
annotations:
  seccomp.security.alpha.kubernetes.io/pod: docker/default
```

**Remediation:**
Review the Kubernetes documentation and if needed, apply a relevant PodSecurityPolicy.

#### 5.7.3
Apply Security Context to Your Pods and Containers (Not Scored)
<details>
<summary>Rationale</summary>
A security context defines the operating system security settings (uid, gid, capabilities, SELinux role, etc..) applied to a container. When designing your containers and pods, make sure that you configure the security context for your pods, containers, and volumes. A security context is a property defined in the deployment yaml. It controls the security parameters that will be assigned to the pod/container/volume. There are two levels of security context: pod level security context, and container-level security context.
</details>

**Result:** Not Scored

**Audit:**
Review the pod definitions in your cluster and verify that you have security contexts defined as appropriate.

**Remediation:**
Follow the Kubernetes documentation and apply security contexts to your pods. For a suggested list of security contexts, you may refer to the CIS Security Benchmark.


#### 5.7.4
The default namespace should not be used (Scored)
<details>
<summary>Rationale</summary>
Resources in a Kubernetes cluster should be segregated by namespace, to allow for security controls to be applied at that level and to make it easier to manage resources.
</details>

**Result:** Pass

**Audit:**
Run the below command on the master node.

```bash
kubectl get all -n default
```

The only entries there should be system-managed resources such as the kubernetes service.

**Remediation:**
By default, K3s does not utilize the default namespace.
