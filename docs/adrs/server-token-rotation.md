# Support Rotating Server Tokens

Date: 2023-08-13

## Status

Accepted 

## Context


### Current tokens

See the existing [Support `kubeadm`-style Bootstrap Token Secrets](agent-join-token.md) ADR 
for more background on current token support.

Important to this discussion is the fact that the `--token` value is used as the passphrase input
used to generate the encryption key for cluster bootstrap data. For this reason, all server nodes 
must use the same token value once the cluster has been started, and the token value cannot be changed.

### Security Considerations

This is a paraphase of @macedogm words concerning the security implications of the current token implementation:

1. Proactiveness: users want to rotate the token periodically as security best practices recommend,
which minimizes the risk of malicious access to the cluster in case the token is leaked. This is 
effective when the rotation happens before a malicious user has the chance to actually use the token.

2. Reactiveness: users want to rotate the token after it's detected that the token leaked. Here we 
should assume the worst case scenario. Similar when the root credentials of a server is leaked and
the server is compromised, we can only be sure of its security and trustworthiness after performing
a clean reinstall (given all the existing stealth rootkits and backdoors that can cause the server to 
be reinfected multiple times). It is assumed that paranoid users and governmental agencies would perform
this action. In this event, token rotation is not enough, only with the clean reinstall one can be 100%
sure of the cluster's security state.

### Existing Work

In past K3s versions, we did not require cluster to be started with a token. When we mandated support
for tokens, we migrated empty string tokens to a randomly generated token. This migration can be
reused to support the rotation from an old token to a new token.


### New Token Rotation

The new token rotation feature will allow the user to rotate the token value used to encrypt cluster bootstrap data.

A new subcommand `k3s token rotate`, will be added to the `k3s` binary. This subcommand can either:
1: take a new supplied token value or 2: Generate a 16 character token and then replace the existing token.
The /var/lib/rancher/k3s/server/token file and passwd file will be updated with the new token value.
Admins can then use the new token value to rejoin existing server nodes or join new server nodes to the cluster.

Similar to the `k3s certificate rotate` and the `k3s secret-encrypt rotate-keys` subcommands,
the `k3s token rotate` subcommand will be wrapper for an API request to the server to perform the decryption
with the old token, and then reencryption of the bootstrap data with the new token. After reenecryption, the 
bootstrap data will be updated with the modified token and password files, allowing propagation of the files to
existing servers upon restart.

### Token Rotation Workflow

HA configuration:

1a) On server 1 run:
```
k3s token rotate -t <OLD_TOKEN>
```

OR
1b)  On server 1 run:

```
k3s token rotate -t <OLD_TOKEN> --new-token <NEW_TOKEN>
```

2) If 1a) Retrieve the new random token value from the /var/lib/rancher/k3s/server/token file on server 1
```
vi /var/lib/rancher/k3s/server/token
```

3) Stop and restart the k3s server process on servers 2 and 3 with the new token value:

```
systemctl stop k3s
# edit /etc/rancher/k3s/config.yaml and update the token value
systemctl start k3s
```

## Decision

We will proceed forward with the above implementation.

## Consequences

Documentation is explicit around what to do if the cluster token is compromised. It's strongly recommend to do a clean cluster reinstall, since this is the only way to be sure of the cluster's security state - eliminating the possibility that backdoors could have been planted by a malicious user.
