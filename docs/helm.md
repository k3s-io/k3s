
# Helm

Helm is the package management tool of choice for Kubernetes. Helm charts provide templating syntax for Kubernetes YAML manifest documents. With Helm we can create configurable deployments instead of just using static files. For more information about creating your own catalog of deployments, check out the docs at [https://helm.sh/docs/intro/quickstart/](https://helm.sh/docs/intro/quickstart/).

K3s does not require any special configuration to use with Helm command-line tools. Just be sure you have properly set up your kubeconfig as per the section about [cluster access](../cluster-access). K3s does include some extra functionality to make deploying both traditional Kubernetes resource manifests and Helm Charts even easier with the [rancher/helm-release CRD.](#using-the-helm-crd)

This section covers the following topics:

- [Automatically Deploying Manifests and Helm Charts](#automatically-deploying-manifests-and-helm-charts)
- [Using the Helm CRD](#using-the-helm-crd)
- [Customizing Packaged Components with HelmChartConfig](#customizing-packaged-components-with-helmchartconfig)
- [Upgrading from Helm v2](#upgrading-from-helm-v2)

### Automatically Deploying Manifests and Helm Charts

Any Kubernetes manifests found in `/var/lib/rancher/k3s/server/manifests` will automatically be deployed to K3s in a manner similar to `kubectl apply`. Manifests deployed in this manner are managed as AddOn custom resources, and can be viewed by running `kubectl get addon -A`. You will find AddOns for packaged components such as CoreDNS, Local-Storage, Traefik, etc. AddOns are created automatically by the deploy controller, and are named based on their filename in the manifests directory.

It is also possible to deploy Helm charts as AddOns. K3s includes a [Helm Controller](https://github.com/rancher/helm-controller/) that manages Helm charts using a HelmChart Custom Resource Definition (CRD).

### Using the Helm CRD

> **Note:** K3s versions through v0.5.0 used `k3s.cattle.io/v1` as the apiVersion for HelmCharts. This has been changed to `helm.cattle.io/v1` for later versions.

The [HelmChart resource definition](https://github.com/rancher/helm-controller#helm-controller) captures most of the options you would normally pass to the `helm` command-line tool. Here's an example of how you might deploy Grafana from the default chart repository, overriding some of the default chart values. Note that the HelmChart resource itself is in the `kube-system` namespace, but the chart's resources will be deployed to the `monitoring` namespace.

```yaml
apiVersion: helm.cattle.io/v1
kind: HelmChart
metadata:
  name: grafana
  namespace: kube-system
spec:
  chart: stable/grafana
  targetNamespace: monitoring
  set:
    adminPassword: "NotVerySafePassword"
  valuesContent: |-
    image:
      tag: master
    env:
      GF_EXPLORE_ENABLED: true
    adminUser: admin
    sidecar:
      datasources:
        enabled: true
```

#### HelmChart Field Definitions

| Field | Default | Description | Helm Argument / Flag Equivalent |
|-------|---------|-------------|-------------------------------|
| name |   | Helm Chart name | NAME |
| spec.chart |   | Helm Chart name in repository, or complete HTTPS URL to chart archive (.tgz) | CHART |
| spec.targetNamespace | default | Helm Chart target namespace | `--namespace` |
| spec.version |   | Helm Chart version (when installing from repository) | `--version` |
| spec.repo |   | Helm Chart repository URL | `--repo` |
| spec.helmVersion | v3 | Helm version to use (`v2` or `v3`) |  |
| spec.bootstrap | False | Set to True if this chart is needed to bootstrap the cluster (Cloud Controller Manager, etc) |  |
| spec.set |   | Override simple default Chart values. These take precedence over options set via valuesContent. | `--set` / `--set-string` |
| spec.jobImage |   | Specify the image to use when installing the helm chart. E.g. rancher/klipper-helm:v0.3.0 . | |
| spec.valuesContent |   | Override complex default Chart values via YAML file content | `--values` |
| spec.chartContent |   | Base64-encoded chart archive .tgz - overrides spec.chart | CHART |

Content placed in `/var/lib/rancher/k3s/server/static/` can be accessed anonymously via the Kubernetes APIServer from within the cluster. This URL can be templated using the special variable `%{KUBERNETES_API}%` in the `spec.chart` field. For example, the packaged Traefik component loads its chart from `https://%{KUBERNETES_API}%/static/charts/traefik-1.81.0.tgz`.

**Note:** The `name` field should follow the Helm chart naming conventions. Refer [here](https://helm.sh/docs/chart_best_practices/conventions/#chart-names) to learn more.

>**Notice on File Naming Requirements:** `HelmChart` and `HelmChartConfig` manifest filenames should adhere to Kubernetes object [naming restrictions](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/). The Helm Controller uses filenames to create objects; therefore, the filename must also align with the restrictions. Any related errors can be observed in the rke2-server logs. The example below is an error generated from using underscores:
```
level=error msg="Failed to process config: failed to process 
/var/lib/rancher/rke2/server/manifests/rke2_ingress_daemonset.yaml: 
Addon.k3s.cattle.io \"rke2_ingress_daemonset\" is invalid: metadata.name: 
Invalid value: \"rke2_ingress_daemonset\": a lowercase RFC 1123 subdomain 
must consist of lower case alphanumeric characters, '-' or '.', and must 
start and end with an alphanumeric character (e.g. 'example.com', regex 
used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]
([-a-z0-9]*[a-z0-9])?)*')"
```

### Customizing Packaged Components with HelmChartConfig

To allow overriding values for packaged components that are deployed as HelmCharts (such as Traefik), K3s versions starting with v1.19.0+k3s1 support customizing deployments via a HelmChartConfig resources. The HelmChartConfig resource must match the name and namespace of its corresponding HelmChart, and supports providing additional `valuesContent`, which is passed to the `helm` command as an additional value file.

> **Note:** HelmChart `spec.set` values override HelmChart and HelmChartConfig `spec.valuesContent` settings.

For example, to customize the packaged Traefik ingress configuration, you can create a file named `/var/lib/rancher/k3s/server/manifests/traefik-config.yaml` and populate it with the following content:

```yaml
apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: traefik
  namespace: kube-system
spec:
  valuesContent: |-
    image: traefik
    imageTag: v1.7.26-alpine
    proxyProtocol:
      enabled: true
      trustedIPs:
        - 10.0.0.0/8
    forwardedHeaders:
      enabled: true
      trustedIPs:
        - 10.0.0.0/8
    ssl:
      enabled: true
      permanentRedirect: false
```

### Upgrading from Helm v2

> **Note:** K3s versions starting with v1.17.0+k3s.1 support Helm v3, and will use it by default. Helm v2 charts can be used by setting `helmVersion: v2` in the spec.

If you were using Helm v2 in previous versions of K3s, you may upgrade to v1.17.0+k3s.1 or newer and Helm 2 will still function. If you wish to migrate to Helm 3, [this](https://helm.sh/blog/migrate-from-helm-v2-to-helm-v3/) blog post by Helm explains how to use a plugin to successfully migrate. Refer to the official Helm 3 documentation [here](https://helm.sh/docs/) for more information. K3s will handle either Helm v2 or Helm v3 as of v1.17.0+k3s.1. Just be sure you have properly set your kubeconfig as per the examples in the section about [cluster access.](../cluster-access)

Note that Helm 3 no longer requires Tiller and the `helm init` command. Refer to the official documentation for details.
