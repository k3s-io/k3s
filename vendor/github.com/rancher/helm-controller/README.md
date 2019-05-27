helm-controller
========
A simple way to manage helm charts with a Custom Resource Definitions in k8s. 

## Manifests and Deploying
The `./manifests` folder contains useful YAML manifests to use for deploying and developing the Helm Controller. This simply YAML deployment creates a HelmChart CRD + a Deployment using the `rancher/helm-controller` container. The YAML might need some modifications for your environment so read below for Namespaced vs Cluster deployments and how to use them properly.

#### Namespaced Deploys
Use the `deploy-namespaced.yaml` to create a namespace and add the Helm Controller and CRD to that namespace locking down the Helm Controller to only see changes to CRDs within that namespace. This is defaulted to `helm-controller` so update the YAML to your needs before running `kubectl create`

#### Cluster Scoped Deploys
If you'd like your helm controller to watch the entire cluster for HelmChart CRD changes use the `deploy-cluster-scoped.yaml` deploy manifest. By default it will add the helm-controller to the `kube-system` so update `metadata.namespace` for your needs.

## Uninstalling
To remove the Helm Controller run `kubectl delete` and pass the deployment YAML used using to create the Deployment `-f` parameter.

## Developing and Building
The Helm Controller is easy to get running locally, follow the instructions for your needs and requires a running k8s server + CRDs etc. When you have a working k8s cluster you can use can use `./manifest/crd.yaml` to create the CRD and `./manifest/example-helmchart.yaml` which runs the `stable/traefik` helm chart.

#### Locally
Building and running natively will start a daemon which will watch a local k8s API. See Manifests section above about how to to create the CRD and Objects using the provided manifests.

```
go build -o ./bin/helm-controller
./bin/helm-controller --kubeconfig $HOME/.kube/config
```

#### docker/k8s
An easy way to get started with docker/k8s is to install docker for windows/mac and use the included k8s cluster. Once functioning you can easily build locally and get a docker container to pull the Helm Controller container and run it in k8s. Use `make` to launch  a linux container and build to create a container. Use the `./manifests/deploy-*.yaml` definitions to get it into your cluster and update  `containers.image` to point to your locally image e.g. `image: rancher/helm-controller:dev`

#### Options and Usage
Use `./bin/helm-controller help` to get full usage details. The outside of a k8s Pod the most important options are `--kubeconfig` or `--masterurl` or it will not run. All options have corresponding ENV variables you could use.

## Testing
`go test ./...`

## License
Copyright (c) 2019 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.