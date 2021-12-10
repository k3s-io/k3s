---
title: "Private Registry Configuration"
weight: 55
---
_Available as of v1.0.0_

Containerd can be configured to connect to private registries and use them to pull private images on the node.

Upon startup, K3s will check to see if a `registries.yaml` file exists at `/etc/rancher/k3s/` and instruct containerd to use any registries defined in the file. If you wish to use a private registry, then you will need to create this file as root on each node that will be using the registry.

Note that server nodes are schedulable by default. If you have not tainted the server nodes and will be running workloads on them, please ensure you also create the `registries.yaml` file on each server as well.

Configuration in containerd can be used to connect to a private registry with a TLS connection and with registries that enable authentication as well. The following section will explain the `registries.yaml` file and give different examples of using private registry configuration in K3s.

# Registries Configuration File

The file consists of two main sections:

- mirrors
- configs

### Mirrors

Mirrors is a directive that defines the names and endpoints of the private registries, for example:

```
mirrors:
  mycustomreg.com:
    endpoint:
      - "https://mycustomreg.com:5000"
```

Each mirror must have a name and set of endpoints. When pulling an image from a registry, containerd will try these endpoint URLs one by one, and use the first working one.

### Configs

The configs section defines the TLS and credential configuration for each mirror. For each mirror you can define `auth` and/or `tls`. The TLS part consists of:

Directive | Description
----------|------------
`cert_file` | The client certificate path that will be used to authenticate with the registry
`key_file` | The client key path that will be used to authenticate with the registry
`ca_file` | Defines the CA certificate path to be used to verify the registry's server cert file
`insecure_skip_verify` | Boolean that defines if TLS verification should be skipped for the registry

The credentials consist of either username/password or authentication token:

- username: user name of the private registry basic auth
- password: user password of the private registry basic auth
- auth: authentication token of the private registry basic auth

Below are basic examples of using private registries in different modes:

### With TLS

Below are examples showing how you may configure `/etc/rancher/k3s/registries.yaml` on each node when using TLS.

{{% tabs %}}
{{% tab "With Authentication" %}}

```
mirrors:
  docker.io:
    endpoint:
      - "https://mycustomreg.com:5000"
configs:
  "mycustomreg:5000":
    auth:
      username: xxxxxx # this is the registry username
      password: xxxxxx # this is the registry password
    tls:
      cert_file: # path to the cert file used in the registry
      key_file:  # path to the key file used in the registry
      ca_file:   # path to the ca file used in the registry
```

{{% /tab %}}
{{% tab "Without Authentication" %}}

```
mirrors:
  docker.io:
    endpoint:
      - "https://mycustomreg.com:5000"
configs:
  "mycustomreg:5000":
    tls:
      cert_file: # path to the cert file used in the registry
      key_file:  # path to the key file used in the registry
      ca_file:   # path to the ca file used in the registry
```

{{% /tab %}}
{{% /tabs %}}

### Without TLS

Below are examples showing how you may configure `/etc/rancher/k3s/registries.yaml` on each node when _not_ using TLS.

{{% tabs %}}
{{% tab "With Authentication" %}}

```
mirrors:
  docker.io:
    endpoint:
      - "http://mycustomreg.com:5000"
configs:
  "mycustomreg:5000":
    auth:
      username: xxxxxx # this is the registry username
      password: xxxxxx # this is the registry password
```

{{% /tab %}}
{{% tab "Without Authentication" %}}

```
mirrors:
  docker.io:
    endpoint:
      - "http://mycustomreg.com:5000"
```

{{% /tab %}}
{{% /tabs %}}

> In case of no TLS communication, you need to specify `http://` for the endpoints, otherwise it will default to https.
 
In order for the registry changes to take effect, you need to restart K3s on each node.

# Adding Images to the Private Registry

First, obtain the k3s-images.txt file from GitHub for the release you are working with.
Pull the K3s images listed on the k3s-images.txt file from docker.io

Example: `docker pull docker.io/rancher/coredns-coredns:1.6.3`

Then, retag the images to the private registry.

Example: `docker tag coredns-coredns:1.6.3 mycustomreg:5000/coredns-coredns`

Last, push the images to the private registry.

Example: `docker push mycustomreg:5000/coredns-coredns`
