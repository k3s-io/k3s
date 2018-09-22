# Kubernetes

<img src="https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png" width="100">

----

Kubernetes without the features I don't care about.

Build
-----

    # First have sane GOPATH, hopefully you know how to do that
    go build -o k3s
    go build -o kubectl ./cmd/kubectl

Run
---

Run containerd

```bash
# Download and install containerd and runc
sudo curl -fL -o /usr/local/bin/runc https://github.com/opencontainers/runc/releases/download/v1.0.0-rc5/runc.amd64
sudo chmod +x /usr/local/bin/runc

curl -fsL https://github.com/containerd/containerd/releases/download/v1.1.1/containerd-1.1.1.linux-amd64.tar.gz | sudo tar xvf /usr/src/containerd.tgz -C /usr/local/bin bin/ --strip-components=1

# Some CNI
sudo mkdir -p /opt/cni/bin
curl -fsL https://github.com/containernetworking/plugins/releases/download/v0.7.1/cni-plugins-amd64-v0.7.1.tgz  | sudo tar xvzf - -C /opt/cni/bin ./loopback

sudo containerd &
```

Run Kubernetes

```bash
# Server
./k3s

# Agent (If doing this on another host copy the ./data folder)
sudo ./k3s agent

# Install Networking
export KUBECONFIG=./data/cred/kubeconfig.yaml
curl -s "https://cloud.weave.works/k8s/net?k8s-version=$(./kubectl version | base64 | tr -d '\n')" | sed 's!rbac.authorization.k8s.io/v1beta1!rbac.authorization.k8s.io/v1!g' | ./kubectl apply -f -
```

Your kubeconfig file is in `./data/cred/kubeconfig.yaml`

Enjoy.
