
# Secrets Encryption

## Secrets Encryption Config

_Available as of v1.17.4+k3s1_

K3s supports enabling secrets encryption at rest by passing the flag `--secrets-encryption` on a server; this flag will do the following automatically:

- Generate an AES-CBC key
- Generate an encryption config file with the generated key
- Pass the config to the KubeAPI as encryption-provider-config

Example of the encryption config file:
```
{
  "kind": "EncryptionConfiguration",
  "apiVersion": "apiserver.config.k8s.io/v1",
  "resources": [
    {
      "resources": [
        "secrets"
      ],
      "providers": [
        {
          "aescbc": {
            "keys": [
              {
                "name": "aescbckey",
                "secret": "xxxxxxxxxxxxxxxxxxx"
              }
            ]
          }
        },
        {
          "identity": {}
        }
      ]
    }
  ]
}
```


## Secrets Encryption Tool
_Available as of v1.21.8+k3s1_

K3s contains a utility tool `secrets-encrypt`, which enables automatic control over the following:

- Disabling/Enabling secrets encryption
- Adding new encryption keys
- Rotating and deleting encryption keys
- Reencrypting secrets

>**Warning:** Failure to follow proper procedure for rotating encryption keys can leave your cluster permanently corrupted. Proceed with caution.

### Single-Server Encryption Key Rotation
To rotate secrets encryption keys on a single-node cluster:

- Start the K3s server with the flag `--secrets-encryption`

>**Note:** Starting K3s without encryption and enabling it at a later time is currently *not* supported.

1. Prepare

    ```
    k3s secrets-encrypt prepare
    ```

2. Kill and restart the K3s server with same arguments
3. Rotate

    ```
    k3s secrets-encrypt rotate
    ```

4. Kill and restart the K3s server with same arguments
5. Reencrypt

    ```
    k3s secrets-encrypt reencrypt
    ```

### High-Availability Encryption Key Rotation
The steps are the same for both embedded DB and external DB clusters.

To rotate secrets encryption keys on HA setups:

>**Notes:** 
>
> - Starting K3s without encryption and enabling it at a later time is currently *not* supported.
>
> - While not required, it is recommended that you pick one server node from which to run the `secrets-encrypt` commands.

- Start up all three K3s servers with the `--secrets-encryption` flag. For brevity, the servers will be referred to as S1, S2, S3.

1. Prepare on S1

    ```
    k3s secrets-encrypt prepare
    ```

2. Kill and restart S1 with same arguments
3. Once S1 is up, kill and restart the S2 and S3

4. Rotate on S1

    ```
    k3s secrets-encrypt rotate
    ```

5. Kill and restart S1 with same arguments
6. Once S1 is up, kill and restart the S2 and S3

7. Reencrypt on S1

    ```
    k3s secrets-encrypt reencrypt
    ```

8. Kill and restart S1 with same arguments
9. Once S1 is up, kill and restart the S2 and S3

### Single-Server Secrets Encryption Disable/Enable
After launching a server with `--secrets-encryption` flag, secrets encryption can be disabled.

To disable secrets encryption on a single-node cluster:

1. Disable

    ```
    k3s secrets-encrypt disable
    ```

2. Kill and restart the K3s server with same arguments

3. Reencrypt with flags

    ```
    k3s secrets-encrypt reencrypt --force --skip
    ```

To re-enable secrets encryption on a single node cluster:

1. Enable

    ```
    k3s secrets-encrypt enable
    ```

2. Kill and restart the K3s server with same arguments

3. Reencrypt with flags

    ```
    k3s secrets-encrypt reencrypt --force --skip
    ```

### High-Availability Secrets Encryption Disable/Enable
After launching a HA cluster with `--secrets-encryption` flags, secrets encryption can be disabled.
>**Note:** While not required, it is recommended that you pick one server node from which to run the `secrets-encrypt` commands.

For brevity, the three servers used in this guide will be referred to as S1, S2, S3.

To disable secrets encryption on a HA cluster:

1. Disable on S1

    ```
    k3s secrets-encrypt disable
    ```

2. Kill and restart S1 with same arguments
3. Once S1 is up, kill and restart the S2 and S3


4. Reencrypt with flags on S1

    ```
    k3s secrets-encrypt reencrypt --force --skip
    ```

To re-enable secrets encryption on a HA cluster:

1. Enable on S1

    ```
    k3s secrets-encrypt enable
    ```

2. Kill and restart S1 with same arguments
3. Once S1 is up, kill and restart the S2 and S3

4. Reencrypt with flags on S1

    ```
    k3s secrets-encrypt reencrypt --force --skip
    ```


### Secrets Encryption Status
The secrets-encrypt tool includes a `status` command that displays information about the current status of secrets encryption on the node.

An example of the command on a single-server node:  
```
$ k3s secrets-encrypt status
Encryption Status: Enabled
Current Rotation Stage: start
Server Encryption Hashes: All hashes match

Active  Key Type  Name
------  --------  ----
 *      AES-CBC   aescbckey

```

Another example on HA cluster, after rotating the keys, but before restarting the servers:  
```
$ k3s secrets-encrypt status
Encryption Status: Enabled
Current Rotation Stage: rotate
Server Encryption Hashes: hash does not match between node-1 and node-2

Active  Key Type  Name
------  --------  ----
 *      AES-CBC   aescbckey-2021-12-10T22:54:38Z
        AES-CBC   aescbckey

```

Details on each section are as follows:  

- __Encryption Status__: Displayed whether secrets encryption is disabled or enabled on the node  
- __Current Rotation Stage__: Indicates the current rotation stage on the node.  
  Stages are: `start`, `prepare`, `rotate`, `reencrypt_request`, `reencrypt_active`, `reencrypt_finished`  
- __Server Encryption Hashes__: Useful for HA clusters, this indicates whether all servers are on the same stage with their local files. This can be used to identify whether a restart of servers is required before proceeding to the next stage. In the HA example above, node-1 and node-2 have different hashes, indicating that they currently do not have the same encryption configuration. Restarting the servers will sync up their configuration.
- __Key Table__: Summarizes information about the secrets encryption keys found on the node.  
  * __Active__: The "*" indicates which, if any, of the keys are currently used for secrets encryption. An active key is used by Kubernetes to encrypt any new secrets.
  * __Key Type__: All keys using this tool are `AES-CBC` type. See more info [here.](https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/#providers)
  * __Name__: Name of the encryption key.  