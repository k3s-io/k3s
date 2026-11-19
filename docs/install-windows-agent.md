# Running the K3s agent as a Windows service

The PowerShell script in
[`contrib/windows/k3s-agent-service.ps1`](../contrib/windows/k3s-agent-service.ps1)
installs the Windows K3s agent as the `k3s-agent` service. It uses
[Shawl](https://github.com/mtkennerly/shawl) as the service wrapper; K3s does
not implement its own Windows service manager.

## Build the Windows binary

The service script does not build K3s. Build a Windows/AMD64 `k3s.exe` from
the same K3s version as the server. Run this on a Linux build host with Docker
Buildx configured:

```sh
export K3S_VERSION='v1.37.1+k3s1' # Set this to the version of your K3s server.
git clone --depth 1 --branch "$K3S_VERSION" \
  https://github.com/k3s-io/k3s.git /tmp/k3s-release-source
cd /tmp/k3s-release-source
docker buildx build --platform=windows/amd64 --target=result \
  --build-arg GIT_TAG="$K3S_VERSION" \
  --output=type=local,dest=/tmp/k3s-release-artifacts -f Dockerfile .
```

The output binary is
`/tmp/k3s-release-artifacts/dist/artifacts/k3s.exe`. Copy it to the Windows
host alongside the service script and `config.yaml`.

## Create the agent configuration

On the K3s server, retrieve its node token:

```sh
sudo cat /var/lib/rancher/k3s/server/node-token
```

On Windows, create `config.yaml` alongside the script and `k3s.exe`. Replace
the placeholders with the server's reachable URL and the token from the
previous command:

```yaml
server: "https://<server-address>:6443"
token: "<server-node-token>"
```

If the Windows host has multiple network adapters, add `node-ip` with the
address reachable by the K3s server and `flannel-iface` with the matching
Windows adapter name. `node-ip` selects the node's advertised address;
`flannel-iface` selects the interface Flannel uses for inter-node traffic.
For a single-adapter host, leave these optional settings out unless your
network requires them.

```yaml
node-ip: "<windows-node-ip>"
flannel-iface: "<windows-adapter-name>"
```

For example, if the node address is `192.168.122.115` and the adapter is named
`Ethernet Instance 0`:

```yaml
node-ip: "192.168.122.115"
flannel-iface: "Ethernet Instance 0"
```

Use `Get-NetIPAddress -AddressFamily IPv4` and `Get-NetAdapter` in PowerShell
to find the address and exact adapter name.

## Install

Place `k3s.exe`, `config.yaml`, and the script in the same directory. In
PowerShell running as Administrator:

```powershell
.\k3s-agent-service.ps1 -K3sPath .\k3s.exe -ConfigFile .\config.yaml
```

The script installs K3s under `C:\Program Files\K3s`, data and the protected
config under `C:\ProgramData\k3s`, and logs under `C:\ProgramData\k3s\logs`.
It registers the service for automatic startup and checks that it remains
running briefly after installation.

Check the Windows service with:

```powershell
Get-Service k3s-agent
```

Service state alone does not mean the node is healthy. Check the node on the
K3s server:

```sh
sudo k3s kubectl get nodes -o wide
```

## Stop and start

```powershell
Stop-Service k3s-agent
Start-Service k3s-agent
```

Shawl sends Ctrl+C on stop and waits up to two minutes for K3s to exit before
terminating its process tree.

## Uninstall the service

This removes the Windows service only; K3s data, config, binaries, and logs are
preserved:

```powershell
.\k3s-agent-service.ps1 -Action Uninstall
```
