# Support etcd Snapshot Configuration via Kubernetes Secret

Date: 2024-02-06
Revised: 2024-06-10

## Status

Accepted

## Context

### Current State

K3s currently reads configuration for S3 storage of etcd snapshots from CLI flags and/or configuration files.

Security-conscious users have raised issue with the current state. They want to store snapshots on S3, but do not want
to have credentials visible in config files or systemd units. Users operating in highly secure environments have also
asked for the ability to configure a proxy server to be used when creating/restoring snapshots stored on S3, without
managing complicated `NO_PROXY` settings or affecting the rest of the K3s process environment.

### Security Considerations

Storing credentials on-disk is generally considered a bad idea, and is not allowed by security practices in many
organizations. Use of static credentials in the config file also makes them difficult to rotate, as K3s only reloads the
configuration on startup.

### Existing Work

Cloud-providers and other tools that need to auth to external systems frequently can be configured to retrieve secrets
from an existing credential secret that is provisioned via an external process, such as a secrets management tool. This
avoids embedding the credentials directly in the system configuration, chart values, and so on.

## Decision

* We will add a `--etcd-s3-proxy` flag that can be used to set the proxy used by the S3 client. This will override the
  settings that golang's default HTTP client reads from the `HTTP_PROXY/HTTPS_PROXY/NO_PROXY` environment varibles.
* We will add support for reading etcd snapshot S3 configuration from a Secret. The secret name will be specified via a new
  `--etcd-s3-config-secret` flag, which accepts the name of the Secret in the `kube-system` namespace.
* Presence of the `--etcd-s3-config-secret` flag does not imply `--etcd-s3`. If S3 is not enabled by use of the `--etcd-s3` flag,
  the Secret will not be used.
* The Secret does not need to exist when K3s starts; it will be checked for every time a snapshot operation is performed.
* Secret and CLI/config values will NOT be merged. The Secret will provide values to be used in absence of other
  configuration; if S3 configuration is passed via CLI flags or configuration file, ALL fields set by the Secret
  will be ignored.
* The Secret will ONLY be used for on-demand and scheduled snapshot save operations; it will not be used by snapshot restore.
  Snapshot restore operations that want to retrieve a snapshot from S3 will need to pass the appropriate configuration
  via environment variables or CLI flags, as the Secret is not available during the restore process.

Fields within the Secret will match `k3s server` CLI flags / config file keys. For the `etcd-s3-endpoint-ca`, which
normally contains the path of a file on disk, the `etcd-s3-endpoint-ca` field can specify an inline PEM-encoded CA
bundle, or the `etcd-s3-endpoint-ca-name` can be used to specify the name of a ConfigMap in the `kube-system` namespace
containing one or more CA bundles. All valid CA bundles found in either field are loaded.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: k3s-etcd-snapshot-s3-config
  namespace: kube-system
stringData:
  etcd-s3-endpoint: ""
  etcd-s3-endpoint-ca: ""
  etcd-s3-endpoint-ca-name: ""
  etcd-s3-skip-ssl-verify: "false"
  etcd-s3-access-key: "AWS_ACCESS_KEY_ID"
  etcd-s3-secret-key: "AWS_SECRET_ACCESS_KEY"
  etcd-s3-bucket: "bucket"
  etcd-s3-folder: "folder"
  etcd-s3-region: "us-east-1"
  etcd-s3-insecure: "false"
  etcd-s3-timeout: "5m"
  etcd-s3-proxy: ""
```

## Consequences

This will require additional documentation, tests, and QA work to validate use of secrets for s3 snapshot configuration.

## Revisions

#### 2024-06-10:
* Changed flag to `etcd-s3-config-secret` to avoid confusion with `etcd-s3-secret-key`.
* Added `etcd-s3-folder` to example Secret.
