---
title: "CIS Hardening Guide"
weight: 80
---

This document provides prescriptive guidance for hardening a production installation of K3s. It outlines the configurations and controls required to address Kubernetes benchmark controls from the Center for Information Security (CIS).

K3s has a number of security mitigations applied and turned on by default and will pass a number of the Kubernetes CIS controls without modification. There are some notable exceptions to this that require manual intervention to fully comply with the CIS Benchmark:

1. K3s will not modify the host operating system. Any host-level modifications will need to be done manually.
2. Certain CIS policy controls for PodSecurityPolicies and NetworkPolicies will restrict the functionality of this cluster. You must opt into having K3s configure these by adding the appropriate options (enabling of admission plugins) to your command-line flags or configuration file as well as manually applying appropriate policies. Further detail in the sections below.

The first section (1.1) of the CIS Benchmark concerns itself primarily with pod manifest permissions and ownership. K3s doesn't utilize these for the core components since everything is packaged into a single binary.

## Host-level Requirements

There are two areas of host-level requirements: kernel parameters and etcd process/directory configuration. These are outlined in this section.

### Ensure `protect-kernel-defaults` is set

This is a kubelet flag that will cause the kubelet to exit if the required kernel parameters are unset or are set to values that are different from the kubelet's defaults.

> **Note:** `protect-kernel-defaults` is exposed as a top-level flag for K3s.

#### Set kernel parameters

Create a file called `/etc/sysctl.d/90-kubelet.conf` and add the snippet below. Then run `sysctl -p /etc/sysctl.d/90-kubelet.conf`.

```bash
vm.panic_on_oom=0
vm.overcommit_memory=1
kernel.panic=10
kernel.panic_on_oops=1
```

## Kubernetes Runtime Requirements

The runtime requirements to comply with the CIS Benchmark are centered around pod security (PSPs) and network policies. These are outlined in this section. K3s doesn't apply any default PSPs or network policies however K3s ships with a controller that is meant to apply a given set of network policies. By default, K3s runs with the "NodeRestriction" admission controller. To enable PSPs, add the following to the K3s start command: `--kube-apiserver-arg="enable-admission-plugins=NodeRestriction,PodSecurityPolicy,ServiceAccount"`. This will have the effect of maintaining the "NodeRestriction" plugin as well as enabling the "PodSecurityPolicy".

### PodSecurityPolicies

When PSPs are enabled, a policy can be applied to satisfy the necessary controls described in section 5.2 of the CIS Benchmark.

Here's an example of a compliant PSP.

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: cis1.5-compliant-psp
spec:
  privileged: false                # CIS - 5.2.1
  allowPrivilegeEscalation: false  # CIS - 5.2.5
  requiredDropCapabilities:        # CIS - 5.2.7/8/9
    - ALL
  volumes:
    - 'configMap'
    - 'emptyDir'
    - 'projected'
    - 'secret'
    - 'downwardAPI'
    - 'persistentVolumeClaim'
  hostNetwork: false               # CIS - 5.2.4
  hostIPC: false                   # CIS - 5.2.3
  hostPID: false                   # CIS - 5.2.2
  runAsUser:
    rule: 'MustRunAsNonRoot'       # CIS - 5.2.6
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'MustRunAs'
    ranges:
      - min: 1
        max: 65535
  fsGroup:
    rule: 'MustRunAs'
    ranges:
      - min: 1
        max: 65535
  readOnlyRootFilesystem: false
```

Before the above PSP to be effective, we need to create a couple ClusterRoles and ClusterRole. We also need to include a "system unrestricted policy" which is needed for system-level pods that require additional privileges.

These can be combined with the PSP yaml above and NetworkPolicy yaml below into a single file and placed in the `/var/lib/rancher/k3s/server/manifests` directory. Below is an example of a `policy.yaml` file. 

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: cis1.5-compliant-psp
spec:
  privileged: false
  allowPrivilegeEscalation: false
  requiredDropCapabilities:
    - ALL
  volumes:
    - 'configMap'
    - 'emptyDir'
    - 'projected'
    - 'secret'
    - 'downwardAPI'
    - 'persistentVolumeClaim'
  hostNetwork: false
  hostIPC: false
  hostPID: false
  runAsUser:
    rule: 'MustRunAsNonRoot'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'MustRunAs'
    ranges:
      - min: 1
        max: 65535
  fsGroup:
    rule: 'MustRunAs'
    ranges:
      - min: 1
        max: 65535
  readOnlyRootFilesystem: false
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: psp:restricted
  labels:
    addonmanager.kubernetes.io/mode: EnsureExists
rules:
- apiGroups: ['extensions']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - cis1.5-compliant-psp
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: default:restricted
  labels:
    addonmanager.kubernetes.io/mode: EnsureExists
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: psp:restricted
subjects:
- kind: Group
  name: system:authenticated
  apiGroup: rbac.authorization.k8s.io
---
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: intra-namespace
  namespace: kube-system
spec:
  podSelector: {}
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: kube-system
---
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: intra-namespace
  namespace: default
spec:
  podSelector: {}
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: default
---
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: intra-namespace
  namespace: kube-public
spec:
  podSelector: {}
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: kube-public
---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: system-unrestricted-psp
spec:
  allowPrivilegeEscalation: true
  allowedCapabilities:
  - '*'
  fsGroup:
    rule: RunAsAny
  hostIPC: true
  hostNetwork: true
  hostPID: true
  hostPorts:
  - max: 65535
    min: 0
  privileged: true
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - '*'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-unrestricted-node-psp-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system-unrestricted-psp-role
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:nodes
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system-unrestricted-psp-role
rules:
- apiGroups:
  - policy
  resourceNames:
  - system-unrestricted-psp
  resources:
  - podsecuritypolicies
  verbs:
  - use
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system-unrestricted-svc-acct-psp-rolebinding
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system-unrestricted-psp-role
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: Group
  name: system:serviceaccounts
```

> **Note:** The Kubernetes critical additions such as CNI, DNS, and Ingress are ran as pods in the `kube-system` namespace. Therefore, this namespace will have a policy that is less restrictive so that these components can run properly.

### NetworkPolicies

> NOTE: K3s deploys kube-router for network policy enforcement. Support for this in K3s is currently experimental.

CIS requires that all namespaces have a network policy applied that reasonably limits traffic into namespaces and pods.

Here's an example of a compliant network policy. 

```yaml
kind: NetworkPolicy
apiVersion: networking.k8s.io/v1
metadata:
  name: intra-namespace
  namespace: kube-system
spec:
  podSelector: {}
  ingress:
    - from:
      - namespaceSelector:
          matchLabels:
            name: kube-system
```

With the applied restrictions, DNS will be blocked unless purposely allowed. Below is a network policy that will allow for traffic to exist for DNS.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: default-network-dns-policy
  namespace: <NAMESPACE>
spec:
  ingress:
  - ports:
    - port: 53
      protocol: TCP
    - port: 53
      protocol: UDP
  podSelector:
    matchLabels:
      k8s-app: kube-dns
  policyTypes:
  - Ingress
```

The metrics-server and Traefik ingress controller will be blocked by default if network policies are not created to allow access. Traefik v1 as packaged in K3s version 1.20 and below uses different labels than Traefik v2; ensure that you only use the sample yaml below that is associated with the version of Traefik present on your cluster.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-metrics-server
  namespace: kube-system
spec:
  podSelector:
    matchLabels:
      k8s-app: metrics-server
  ingress:
  - {}
  policyTypes:
  - Ingress
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-svclbtraefik-ingress
  namespace: kube-system
spec:
  podSelector: 
    matchLabels:
      app: svclb-traefik
  ingress:
  - {}
  policyTypes:
  - Ingress
---
# Below is for 1.20 ONLY -- remove if on 1.21 or above
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-traefik-v120-ingress
  namespace: kube-system
spec:
  podSelector:
    matchLabels:
      app: traefik
  ingress:
  - {}
  policyTypes:
  - Ingress
---
# Below is for 1.21 and above ONLY -- remove if on 1.20 or below
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-all-traefik-v121-ingress
  namespace: kube-system
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: traefik
  ingress:
  - {}
  policyTypes:
  - Ingress
```

> **Note:** Operators must manage network policies as normal for additional namespaces that are created.

## Known Issues
The following are controls that K3s currently does not pass by default. Each gap will be explained, along with a note clarifying whether it can be passed through manual operator intervention, or if it will be addressed in a future release of K3s.


### Control 1.2.15
Ensure that the admission control plugin `NamespaceLifecycle` is set.
<details>
<summary>Rationale</summary>
Setting admission control policy to NamespaceLifecycle ensures that objects cannot be created in non-existent namespaces, and that namespaces undergoing termination are not used for creating the new objects. This is recommended to enforce the integrity of the namespace termination process and also for the availability of the newer objects.

This can be remediated by passing this argument as a value to the `enable-admission-plugins=` and pass that to  `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.16 (mentioned above)
Ensure that the admission control plugin `PodSecurityPolicy` is set.
<details>
<summary>Rationale</summary>
A Pod Security Policy is a cluster-level resource that controls the actions that a pod can perform and what it has the ability to access. The PodSecurityPolicy objects define a set of conditions that a pod must run with in order to be accepted into the system. Pod Security Policies are comprised of settings and strategies that control the security features a pod has access to and hence this must be used to control pod access permissions.

This can be remediated by passing this argument as a value to the `enable-admission-plugins=` and pass that to  `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.22
Ensure that the `--audit-log-path` argument is set.
<details>
<summary>Rationale</summary>
Auditing the Kubernetes API Server provides a security-relevant chronological set of records documenting the sequence of activities that have affected system by individual users, administrators or other components of the system. Even though currently, Kubernetes provides only basic audit capabilities, it should be enabled. You can enable it by setting an appropriate audit log path.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.23
Ensure that the `--audit-log-maxage` argument is set to 30 or as appropriate.
<details>
<summary>Rationale</summary>
Retaining logs for at least 30 days ensures that you can go back in time and investigate or correlate any events. Set your audit log retention period to 30 days or as per your business requirements.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.24
Ensure that the `--audit-log-maxbackup` argument is set to 10 or as appropriate.
<details>
<summary>Rationale</summary>
Kubernetes automatically rotates the log files. Retaining old log files ensures that you would have sufficient log data available for carrying out any investigation or correlation. For example, if you have set file size of 100 MB and the number of old log files to keep as 10, you would approximate have 1 GB of log data that you could potentially use for your analysis.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.25
Ensure that the `--audit-log-maxsize` argument is set to 100 or as appropriate.
<details>
<summary>Rationale</summary>
Kubernetes automatically rotates the log files. Retaining old log files ensures that you would have sufficient log data available for carrying out any investigation or correlation. If you have set file size of 100 MB and the number of old log files to keep as 10, you would approximate have 1 GB of log data that you could potentially use for your analysis.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.26
Ensure that the `--request-timeout` argument is set as appropriate.
<details>
<summary>Rationale</summary>
Setting global request timeout allows extending the API server request timeout limit to a duration appropriate to the user's connection speed. By default, it is set to 60 seconds which might be problematic on slower connections making cluster resources inaccessible once the data volume for requests exceeds what can be transmitted in 60 seconds. But, setting this timeout limit to be too large can exhaust the API server resources making it prone to Denial-of-Service attack. Hence, it is recommended to set this limit as appropriate and change the default limit of 60 seconds only if needed.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.27
Ensure that the `--service-account-lookup` argument is set to true.
<details>
<summary>Rationale</summary>
If `--service-account-lookup` is not enabled, the apiserver only verifies that the authentication token is valid, and does not validate that the service account token mentioned in the request is actually present in etcd. This allows using a service account token even after the corresponding service account is deleted. This is an example of time of check to time of use security issue.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 1.2.33
Ensure that the `--encryption-provider-config` argument is set as appropriate.
<details>
<summary>Rationale</summary>
Where `etcd` encryption is used, it is important to ensure that the appropriate set of encryption providers is used. Currently, the aescbc, kms and secretbox are likely to be appropriate options.
</details>

### Control 1.2.34
Ensure that encryption providers are appropriately configured.
<details>
<summary>Rationale</summary>
`etcd` is a highly available key-value store used by Kubernetes deployments for persistent storage of all of its REST API objects. These objects are sensitive in nature and should be encrypted at rest to avoid any disclosures.

This can be remediated by passing a valid configuration to `k3s` as outlined above.
</details>

### Control 1.3.1
Ensure that the `--terminated-pod-gc-threshold` argument is set as appropriate.
<details>
<summary>Rationale</summary>
Garbage collection is important to ensure sufficient resource availability and avoiding degraded performance and availability. In the worst case, the system might crash or just be unusable for a long period of time. The current setting for garbage collection is 12,500 terminated pods which might be too high for your system to sustain. Based on your system resources and tests, choose an appropriate threshold value to activate garbage collection.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 3.2.1
Ensure that a minimal audit policy is created (Scored)
<details>
<summary>Rationale</summary>
Logging is an important detective control for all systems, to detect potential unauthorized access.

This can be remediated by passing controls 1.2.22 - 1.2.25 and verifying their efficacy.
</details>


### Control 4.2.7
Ensure that the `--make-iptables-util-chains` argument is set to true.
<details>
<summary>Rationale</summary>
Kubelets can automatically manage the required changes to iptables based on how you choose your networking options for the pods. It is recommended to let kubelets manage the changes to iptables. This ensures that the iptables configuration remains in sync with pods networking configuration. Manually configuring iptables with dynamic pod network configuration changes might hamper the communication between pods/containers and to the outside world. You might have iptables rules too restrictive or too open.

This can be remediated by passing this argument as a value to the `--kube-apiserver-arg=` argument to `k3s server`. An example can be found below.
</details>

### Control 5.1.5
Ensure that default service accounts are not actively used. (Scored)
<details>
<summary>Rationale</summary>

Kubernetes provides a default service account which is used by cluster workloads where no specific service account is assigned to the pod.

Where access to the Kubernetes API from a pod is required, a specific service account should be created for that pod, and rights granted to that service account.

The default service account should be configured such that it does not provide a service account token and does not have any explicit rights assignments.
</details>

The remediation for this is to update the `automountServiceAccountToken` field to `false` for the `default` service account in each namespace.

For `default` service accounts in the built-in namespaces (`kube-system`, `kube-public`, `kube-node-lease`, and `default`), K3s does not automatically do this. You can manually update this field on these service accounts to pass the control.

## Control Plane Execution and Arguments

Listed below are the K3s control plane components and the arguments they're given at start, by default. Commented to their right is the CIS 1.5 control that they satisfy.

```bash
kube-apiserver 
    --advertise-port=6443 
    --allow-privileged=true 
    --anonymous-auth=false                                                            # 1.2.1
    --api-audiences=unknown 
    --authorization-mode=Node,RBAC 
    --bind-address=127.0.0.1 
    --cert-dir=/var/lib/rancher/k3s/server/tls/temporary-certs
    --client-ca-file=/var/lib/rancher/k3s/server/tls/client-ca.crt                    # 1.2.31
    --enable-admission-plugins=NodeRestriction,PodSecurityPolicy                      # 1.2.17
    --etcd-cafile=/var/lib/rancher/k3s/server/tls/etcd/server-ca.crt                  # 1.2.32
    --etcd-certfile=/var/lib/rancher/k3s/server/tls/etcd/client.crt                   # 1.2.29
    --etcd-keyfile=/var/lib/rancher/k3s/server/tls/etcd/client.key                    # 1.2.29
    --etcd-servers=https://127.0.0.1:2379 
    --insecure-port=0                                                                 # 1.2.19
    --kubelet-certificate-authority=/var/lib/rancher/k3s/server/tls/server-ca.crt 
    --kubelet-client-certificate=/var/lib/rancher/k3s/server/tls/client-kube-apiserver.crt 
    --kubelet-client-key=/var/lib/rancher/k3s/server/tls/client-kube-apiserver.key 
    --profiling=false                                                                 # 1.2.21
    --proxy-client-cert-file=/var/lib/rancher/k3s/server/tls/client-auth-proxy.crt 
    --proxy-client-key-file=/var/lib/rancher/k3s/server/tls/client-auth-proxy.key 
    --requestheader-allowed-names=system:auth-proxy 
    --requestheader-client-ca-file=/var/lib/rancher/k3s/server/tls/request-header-ca.crt 
    --requestheader-extra-headers-prefix=X-Remote-Extra- 
    --requestheader-group-headers=X-Remote-Group 
    --requestheader-username-headers=X-Remote-User 
    --secure-port=6444                                                                # 1.2.20
    --service-account-issuer=k3s 
    --service-account-key-file=/var/lib/rancher/k3s/server/tls/service.key            # 1.2.28
    --service-account-signing-key-file=/var/lib/rancher/k3s/server/tls/service.key 
    --service-cluster-ip-range=10.43.0.0/16 
    --storage-backend=etcd3 
    --tls-cert-file=/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.crt        # 1.2.30
    --tls-private-key-file=/var/lib/rancher/k3s/server/tls/serving-kube-apiserver.key # 1.2.30
```

```bash
kube-controller-manager 
    --address=127.0.0.1 
    --allocate-node-cidrs=true 
    --bind-address=127.0.0.1                                                       # 1.3.7
    --cluster-cidr=10.42.0.0/16 
    --cluster-signing-cert-file=/var/lib/rancher/k3s/server/tls/client-ca.crt 
    --cluster-signing-key-file=/var/lib/rancher/k3s/server/tls/client-ca.key 
    --kubeconfig=/var/lib/rancher/k3s/server/cred/controller.kubeconfig 
    --port=10252 
    --profiling=false                                                              # 1.3.2
    --root-ca-file=/var/lib/rancher/k3s/server/tls/server-ca.crt                   # 1.3.5
    --secure-port=0 
    --service-account-private-key-file=/var/lib/rancher/k3s/server/tls/service.key # 1.3.4 
    --use-service-account-credentials=true                                         # 1.3.3
```

```bash
kube-scheduler 
    --address=127.0.0.1 
    --bind-address=127.0.0.1                                              # 1.4.2
    --kubeconfig=/var/lib/rancher/k3s/server/cred/scheduler.kubeconfig 
    --port=10251 
    --profiling=false                                                     # 1.4.1
    --secure-port=0
```

```bash
kubelet 
    --address=0.0.0.0 
    --anonymous-auth=false                                                # 4.2.1
    --authentication-token-webhook=true 
    --authorization-mode=Webhook                                          # 4.2.2
    --cgroup-driver=cgroupfs 
    --client-ca-file=/var/lib/rancher/k3s/agent/client-ca.crt             # 4.2.3
    --cloud-provider=external 
    --cluster-dns=10.43.0.10 
    --cluster-domain=cluster.local 
    --cni-bin-dir=/var/lib/rancher/k3s/data/223e6420f8db0d8828a8f5ed3c44489bb8eb47aa71485404f8af8c462a29bea3/bin 
    --cni-conf-dir=/var/lib/rancher/k3s/agent/etc/cni/net.d 
    --container-runtime-endpoint=/run/k3s/containerd/containerd.sock 
    --container-runtime=remote 
    --containerd=/run/k3s/containerd/containerd.sock 
    --eviction-hard=imagefs.available<5%,nodefs.available<5% 
    --eviction-minimum-reclaim=imagefs.available=10%,nodefs.available=10% 
    --fail-swap-on=false 
    --healthz-bind-address=127.0.0.1 
    --hostname-override=hostname01 
    --kubeconfig=/var/lib/rancher/k3s/agent/kubelet.kubeconfig 
    --kubelet-cgroups=/systemd/system.slice 
    --node-labels= 
    --pod-manifest-path=/var/lib/rancher/k3s/agent/pod-manifests 
    --protect-kernel-defaults=true                                        # 4.2.6
    --read-only-port=0                                                    # 4.2.4
    --resolv-conf=/run/systemd/resolve/resolv.conf 
    --runtime-cgroups=/systemd/system.slice 
    --serialize-image-pulls=false 
    --tls-cert-file=/var/lib/rancher/k3s/agent/serving-kubelet.crt        # 4.2.10
    --tls-private-key-file=/var/lib/rancher/k3s/agent/serving-kubelet.key # 4.2.10
```

The command below is an example of how the outlined remediations can be applied.

```bash
k3s server \
    --protect-kernel-defaults=true \
    --secrets-encryption=true \
    --kube-apiserver-arg='audit-log-path=/var/lib/rancher/k3s/server/logs/audit-log' \
    --kube-apiserver-arg='audit-log-maxage=30' \
    --kube-apiserver-arg='audit-log-maxbackup=10' \
    --kube-apiserver-arg='audit-log-maxsize=100' \
    --kube-apiserver-arg='request-timeout=300s' \
    --kube-apiserver-arg='service-account-lookup=true' \
    --kube-apiserver-arg='enable-admission-plugins=NodeRestriction,PodSecurityPolicy,NamespaceLifecycle,ServiceAccount' \
    --kube-controller-manager-arg='terminated-pod-gc-threshold=10' \
    --kube-controller-manager-arg='use-service-account-credentials=true' \
    --kubelet-arg='streaming-connection-idle-timeout=5m' \
    --kubelet-arg='make-iptables-util-chains=true'
```

## Conclusion

If you have followed this guide, your K3s cluster will be configured to comply with the CIS Kubernetes Benchmark. You can review the [CIS Benchmark Self-Assessment Guide](../self_assessment/) to understand the expectations of each of the benchmarks and how you can do the same on your cluster.
