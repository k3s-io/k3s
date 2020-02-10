k3s - Kubernetes Dashboard
==========================
This is a quick-start summary from the [Kubernetes Dashboard](https://github.com/kubernetes/dashboard/blob/master/docs/user/access-control/creating-sample-user.md) documentation.

>IMPORTANT: Granting admin privileges to the Dashboard's Service Account might be a security risk!

Quick-Start
-----------
>Note: Depending on your version of k3s, you may need to execute the following commands with `sudo`.

**Deploy the Kubernetes Dashboard**

```bash
kubectl create -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.0.0-rc5/aio/deploy/recommended.yaml
```

**Create and deploy the Dashboard `admin-user` ServiceAccount and ClusterRoleBinding resource manifest files**

`dashboard.admin-user.yml`
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: admin-user
  namespace: kube-system
```

`dashboard.admin-user-role.yml`
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: admin-user
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- kind: ServiceAccount
  name: admin-user
  namespace: kube-system
```

```bash
kubectl create -f dashboard.admin-user.yml -f dashboard.admin-user-role.yml
```

**Obtain the Bearer Token for the `admin-user`**

```bash
kubectl -n kube-system describe secret admin-user-token | grep ^token
```

**Configure the secure channel proxy**

To access the Dashboard you must create a secure channel to your k3s cluster:

```bash
kubectl proxy
```

**Access the Dashboard**

- http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
- `Sign In` with the `admin-user` Bearer Token

**Upgrading the Dashboard**

The latest releases are available from: https://github.com/kubernetes/dashboard/releases/latest

```bash
kubectl delete ns kubernetes-dashboard
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/[...]
```

**Deleting the Dashboard and `admin-user` resources**

```bash
kubectl delete -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.0.0-rc5/aio/deploy/recommended.yaml -f dashboard.admin-user.yml -f dashboard.admin-user-role.yml
```
