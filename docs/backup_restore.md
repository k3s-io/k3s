
# Backup and Restore

The way K3s is backed up and restored depends on which type of datastore is used.

- [Backup and Restore with External Datastore](#backup-and-restore-with-external-datastore)
- [Backup and Restore with Embedded etcd Datastore (Experimental)](#backup-and-restore-with-embedded-etcd-datastore-experimental)

## Backup and Restore with External Datastore

When an external datastore is used, backup and restore operations are handled outside of K3s. The database administrator will need to back up the external database, or restore it from a snapshot or dump.

We recommend configuring the database to take recurring snapshots.

For details on taking database snapshots and restoring your database from them, refer to the official database documentation:

- [Official MySQL documentation](https://dev.mysql.com/doc/refman/8.0/en/replication-snapshot-method.html)
- [Official PostgreSQL documentation](https://www.postgresql.org/docs/8.3/backup-dump.html)
- [Official etcd documentation](https://etcd.io/docs/latest/op-guide/recovery/)

## Backup and Restore with Embedded etcd Datastore (Experimental)

_Available as of v1.19.1+k3s1_

In this section, you'll learn how to create backups of the K3s cluster data and to restore the cluster from backup.

>**Note on Single-Server with embedded SQLite:** Currently, backups of SQLite are not supported. Instead, make a copy of `/var/lib/rancher/k3s/server` and then delete K3s. 

### Creating Snapshots

Snapshots are enabled by default.

The snapshot directory defaults to `${data-dir}/server/db/snapshots`. The data-dir value defaults to `/var/lib/rancher/k3s` and can be changed by setting the `--data-dir` flag.

To configure the snapshot interval or the number of retained snapshots, refer to the [options.](#options)

### Restoring a Cluster from a Snapshot

When K3s is restored from backup, the old data directory will be moved to `${data-dir}/server/db/etcd-old/`. Then K3s will attempt to restore the snapshot by creating a new data directory, then starting etcd with a new K3s cluster with one etcd member.

To restore the cluster from backup, run K3s with the `--cluster-reset` option, with the `--cluster-reset-restore-path` also given:

```
./k3s server \
  --cluster-reset \
  --cluster-reset-restore-path=<PATH-TO-SNAPSHOT>
```

**Result:**  A message in the logs says that K3s can be restarted without the flags. Start k3s again and should run successfully and be restored from the specified snapshot.

### Options

These options can be passed in with the command line, or in the [configuration file,](install/install-options/install_options.md#configuration-file ) which may be easier to use.

| Options | Description |
| ----------- | --------------- |
| `--etcd-disable-snapshots` | Disable automatic etcd snapshots |
| `--etcd-snapshot-schedule-cron` value  |  Snapshot interval time in cron spec. eg. every 5 hours `* */5 * * *`(default: `0 */12 * * *`) |
| `--etcd-snapshot-retention` value  | Number of snapshots to retain (default: 5) |
| `--etcd-snapshot-dir` value  | Directory to save db snapshots. (Default location: `${data-dir}/db/snapshots`) |
| `--cluster-reset`  | Forget all peers and become sole member of a new cluster. This can also be set with the environment variable `[$K3S_CLUSTER_RESET]`.
| `--cluster-reset-restore-path` value | Path to snapshot file to be restored

### S3 Compatible API Support

K3s supports writing etcd snapshots to and restoring etcd snapshots from systems with S3-compatible APIs. S3 support is available for both on-demand and scheduled snapshots.

The arguments below have been added to the `server` subcommand. These flags exist for the `etcd-snapshot` subcommand as well however the `--etcd-s3` portion is removed to avoid redundancy.

| Options | Description |
| ----------- | --------------- |
| `--etcd-s3` | Enable backup to S3 |
| `--etcd-s3-endpoint` | S3 endpoint url |
| `--etcd-s3-endpoint-ca` | S3 custom CA cert to connect to S3 endpoint |
| `--etcd-s3-skip-ssl-verify` | Disables S3 SSL certificate validation |
| `--etcd-s3-access-key` |  S3 access key |
| `--etcd-s3-secret-key` | S3 secret key" |
| `--etcd-s3-bucket` | S3 bucket name |
| `--etcd-s3-region` | S3 region / bucket location (optional). defaults to us-east-1 |
| `--etcd-s3-folder` | S3 folder |

To perform an on-demand etcd snapshot and save it to S3:

```
k3s etcd-snapshot \
  --s3 \
  --s3-bucket=<S3-BUCKET-NAME> \
  --s3-access-key=<S3-ACCESS-KEY> \
  --s3-secret-key=<S3-SECRET-KEY>
```

To perform an on-demand etcd snapshot restore from S3, first make sure that K3s isn't running. Then run the following commands:

```
k3s server \
  --cluster-init \
  --cluster-reset \
  --etcd-s3 \
  --cluster-reset-restore-path=<SNAPSHOT-NAME> \
  --etcd-s3-bucket=<S3-BUCKET-NAME> \
  --etcd-s3-access-key=<S3-ACCESS-KEY> \
  --etcd-s3-secret-key=<S3-SECRET-KEY>
```

### Etcd Snapshot and Restore Subcommands

k3s supports a set of subcommands for working with your etcd snapshots.

| Subcommand | Description |
| ----------- | --------------- |
| delete      |  Delete given snapshot(s) |
| ls, list, l |  List snapshots |
| prune       |  Remove snapshots that exceed the configured retention count |
| save        |  Trigger an immediate etcd snapshot |

*note* The `save` subcommand is the same as `k3s etcd-snapshot`. The latter will eventually be deprecated in favor of the former.

These commands will perform as expected whether the etcd snapshots are stored locally or in an S3 compatible object store.

For additional information on the etcd snapshot subcommands, run `k3s etcd-snapshot`.

Delete a snapshot from S3.

```
k3s etcd-snapshot delete          \
  --s3                            \
  --s3-bucket=<S3-BUCKET-NAME>    \
  --s3-access-key=<S3-ACCESS-KEY> \
  --s3-secret-key=<S3-SECRET-KEY> \
  <SNAPSHOT-NAME>
```

Prune local snapshots with the default retention policy (5). The `prune` subcommand takes an additional flag `--snapshot-retention` that allows for overriding the default retention policy.

```
k3s etcd-snapshot prune
```

```
k3s etcd-snapshot prune --snapshot-retention 10
```
