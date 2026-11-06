[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Medium')]
param(
    [string]$DataDir = $(
        if (-not [string]::IsNullOrWhiteSpace($env:K3S_DATA_DIR)) {
            $env:K3S_DATA_DIR
        } elseif (Test-Path (Join-Path $env:ProgramData 'k3s\data')) {
            Join-Path $env:ProgramData 'k3s\data'
        } else {
            Join-Path $env:SystemDrive 'var\lib\rancher\k3s'
        }
    ),
    [string]$ConfigDir = $(
        $programDataConfig = Join-Path $env:ProgramData 'k3s\config.yaml'
        if (Test-Path $programDataConfig) {
            $programDataConfig
        } else {
            Join-Path $env:SystemDrive 'etc\rancher\k3s'
        }
    ),
    [string]$BinaryPath = (Join-Path $env:ProgramFiles 'K3s\k3s.exe'),
    [string]$WcLayerPath,
    [string]$FlannelNetworkName
)

$ErrorActionPreference = 'Stop'

if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
    throw 'This uninstall script can only run on Windows.'
}

$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
$principal = New-Object Security.Principal.WindowsPrincipal($identity)
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw 'Run this script from an elevated PowerShell session.'
}

function Resolve-RemovalPath {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Path,
        [Parameter(Mandatory = $true)]
        [string]$Name
    )

    $fullPath = [System.IO.Path]::GetFullPath($Path)
    $rootPath = [System.IO.Path]::GetPathRoot($fullPath)
    $trimCharacters = [char[]]@('\', '/')
    if ($fullPath.TrimEnd($trimCharacters) -eq $rootPath.TrimEnd($trimCharacters)) {
        throw "Refusing to remove $Name because '$fullPath' is a drive or share root."
    }

    $protectedPaths = @(
        $env:SystemRoot,
        $env:ProgramFiles,
        ${env:ProgramFiles(x86)},
        $env:ProgramData,
        (Join-Path $systemDrive 'Users')
    ) | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
    foreach ($protectedPath in $protectedPaths) {
        $protectedPath = [System.IO.Path]::GetFullPath($protectedPath).TrimEnd($trimCharacters)
        $targetPath = $fullPath.TrimEnd($trimCharacters)
        if ([string]::Equals($targetPath, $protectedPath, [StringComparison]::OrdinalIgnoreCase) -or
            $protectedPath.StartsWith($targetPath + [System.IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
            throw "Refusing to remove $Name because '$fullPath' contains a protected Windows directory."
        }
    }

    return $fullPath
}

function Remove-K3sService {
    param([string]$Name)

    $service = Get-Service -Name $Name -ErrorAction SilentlyContinue
    if ($null -eq $service) {
        return
    }

    if ($service.Status -ne [System.ServiceProcess.ServiceControllerStatus]::Stopped) {
        if ($service.Status -ne [System.ServiceProcess.ServiceControllerStatus]::StopPending) {
            Stop-Service -Name $Name -Force
        }
        $service.WaitForStatus(
            [System.ServiceProcess.ServiceControllerStatus]::Stopped,
            [TimeSpan]::FromSeconds(120)
        )
    }

    & sc.exe delete $Name
    if ($LASTEXITCODE -ne 0) {
        throw "Windows failed to remove service '$Name' (exit code $LASTEXITCODE)."
    }

    $deadline = [DateTime]::UtcNow.AddSeconds(30)
    while ($null -ne (Get-Service -Name $Name -ErrorAction SilentlyContinue) -and [DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Seconds 1
    }
    if ($null -ne (Get-Service -Name $Name -ErrorAction SilentlyContinue)) {
        throw "Service '$Name' is marked for deletion but remains registered. Retry after Windows releases it."
    }
    Write-Host "Stopped and removed service '$Name'."
}

$systemDrive = if ([string]::IsNullOrWhiteSpace($env:SystemDrive)) { 'C:' } else { $env:SystemDrive }
$programDataK3s = Join-Path $env:ProgramData 'k3s'
$dataDirectory = Resolve-RemovalPath -Path $DataDir -Name 'the K3s data directory'
$configDirectory = Resolve-RemovalPath -Path $ConfigDir -Name 'the K3s configuration path'
$binary = if ([string]::IsNullOrWhiteSpace($BinaryPath)) {
    $null
} else {
    Resolve-RemovalPath -Path $BinaryPath -Name 'the K3s executable'
}

$snapshotDirectory = Join-Path $dataDirectory 'agent\containerd\io.containerd.snapshotter.v1.windows\snapshots'
$snapshots = @()
if (Test-Path -LiteralPath $snapshotDirectory -PathType Container) {
    $snapshots = @(Get-ChildItem -LiteralPath $snapshotDirectory -Directory -Force)
}

if ([string]::IsNullOrWhiteSpace($WcLayerPath)) {
    $WcLayerPath = Join-Path $PSScriptRoot 'wclayer.exe'
    if (-not (Test-Path -LiteralPath $WcLayerPath -PathType Leaf)) {
        $WcLayerPath = Join-Path (Join-Path $env:ProgramFiles 'K3s') 'wclayer.exe'
    }
}
if ($snapshots.Count -gt 0 -and -not $WhatIfPreference -and
    -not (Test-Path -LiteralPath $WcLayerPath -PathType Leaf)) {
    throw "Found HCS-managed Windows snapshots under '$snapshotDirectory'. Build or provide wclayer.exe with -WcLayerPath; no services or files were removed."
}
if (Test-Path -LiteralPath $WcLayerPath -PathType Leaf) {
    $WcLayerPath = (Resolve-Path -LiteralPath $WcLayerPath).Path
}

$flannelNetworks = @()
if (Get-Command -Name Get-HnsNetwork -ErrorAction SilentlyContinue) {
    $availableFlannelNetworks = @(Get-HnsNetwork | Where-Object { $_.Name -like 'flannel.*' })
    if (-not [string]::IsNullOrWhiteSpace($FlannelNetworkName)) {
        $flannelNetworks = @($availableFlannelNetworks | Where-Object { $_.Name -eq $FlannelNetworkName })
        if ($flannelNetworks.Count -eq 0) {
            throw "Flannel HNS network '$FlannelNetworkName' was not found."
        }
    } elseif ($availableFlannelNetworks.Count -gt 1) {
        $names = ($availableFlannelNetworks | ForEach-Object { $_.Name }) -join ', '
        throw "Multiple Flannel HNS networks were found ($names). Specify the K3s network with -FlannelNetworkName; no services or files were removed."
    } else {
        $flannelNetworks = $availableFlannelNetworks
    }
}

$targets = @(
    [pscustomobject]@{ Name = 'K3s data directory'; Path = $dataDirectory },
    [pscustomobject]@{ Name = 'K3s configuration path'; Path = $configDirectory },
    [pscustomobject]@{
        Name = 'K3s node password directory'
        Path = Resolve-RemovalPath -Path (Join-Path $systemDrive 'etc\rancher\node') -Name 'the K3s node password directory'
    },
    [pscustomobject]@{
        Name = 'kubelet directory'
        Path = Resolve-RemovalPath -Path (Join-Path $systemDrive 'var\lib\kubelet') -Name 'the kubelet directory'
    },
    [pscustomobject]@{
        Name = 'CNI state directory'
        Path = Resolve-RemovalPath -Path (Join-Path $systemDrive 'var\lib\cni') -Name 'the CNI state directory'
    },
    [pscustomobject]@{
        Name = 'pod log directory'
        Path = Resolve-RemovalPath -Path (Join-Path $systemDrive 'var\log\pods') -Name 'the pod log directory'
    },
    [pscustomobject]@{
        Name = 'container log directory'
        Path = Resolve-RemovalPath -Path (Join-Path $systemDrive 'var\log\containers') -Name 'the container log directory'
    },
    [pscustomobject]@{
        Name = 'flannel runtime directory'
        Path = Resolve-RemovalPath -Path (Join-Path $systemDrive 'run\flannel') -Name 'the flannel runtime directory'
    },
    [pscustomobject]@{
        Name = 'K3s service data and logs'
        Path = Resolve-RemovalPath -Path $programDataK3s -Name 'the K3s ProgramData directory'
    }
)
if ($null -ne $binary) {
    $targets += [pscustomobject]@{ Name = 'K3s executable'; Path = $binary }
}
$serviceInstallDirectory = Join-Path $env:ProgramFiles 'K3s'
foreach ($serviceFile in @('shawl.exe', 'shawl-LICENSE.txt')) {
    $targets += [pscustomobject]@{
        Name = "K3s service wrapper file '$serviceFile'"
        Path = Resolve-RemovalPath -Path (Join-Path $serviceInstallDirectory $serviceFile) -Name 'a K3s service wrapper file'
    }
}

foreach ($serviceName in @('k3s-agent', 'k3s')) {
    if ($null -ne (Get-Service -Name $serviceName -ErrorAction SilentlyContinue) -and
        $PSCmdlet.ShouldProcess($serviceName, 'Stop and remove K3s service')) {
        Remove-K3sService -Name $serviceName
    }
}

if (-not $WhatIfPreference) {
    $runningK3s = Get-Process -Name 'k3s' -ErrorAction SilentlyContinue
    if ($runningK3s) {
        $processIds = ($runningK3s | ForEach-Object { $_.Id }) -join ', '
        throw "K3s is still running outside the k3s/k3s-agent services (process ID(s): $processIds). Stop it and rerun this script; no files were removed."
    }
}

if ($snapshots.Count -gt 0) {
    $pendingSnapshots = @($snapshots | Sort-Object { [int64]$_.Name } -Descending)
    while ($pendingSnapshots.Count -gt 0) {
        $failedSnapshots = @()
        $removedSnapshot = $false
        foreach ($snapshot in $pendingSnapshots) {
            if (-not $PSCmdlet.ShouldProcess($snapshot.FullName, 'Remove HCS-managed container snapshot')) {
                continue
            }

            & $WcLayerPath remove $snapshot.FullName
            if ($LASTEXITCODE -eq 0) {
                Write-Host "Removed HCS-managed snapshot: $($snapshot.FullName)"
                $removedSnapshot = $true
            } else {
                $failedSnapshots += $snapshot
            }
        }

        if ($WhatIfPreference) {
            break
        }
        if ($failedSnapshots.Count -gt 0 -and -not $removedSnapshot) {
            $paths = ($failedSnapshots | ForEach-Object { $_.FullName }) -join ', '
            throw "wclayer.exe could not remove HCS-managed snapshot(s): $paths. No later targets were removed."
        }
        $pendingSnapshots = $failedSnapshots
    }
}

foreach ($network in $flannelNetworks) {
    if ($PSCmdlet.ShouldProcess($network.Name, 'Remove K3s Flannel HNS network')) {
        Get-HnsEndpoint |
            Where-Object { $_.VirtualNetwork -eq $network.Id } |
            Remove-HnsEndpoint
        $network | Remove-HnsNetwork
        Write-Host "Removed Flannel HNS network: $($network.Name)"
    }
}

$removedTargets = @{}
foreach ($target in $targets) {
    if ($removedTargets.ContainsKey($target.Path)) {
        continue
    }
    if (-not (Test-Path -LiteralPath $target.Path)) {
        Write-Host "Skipping missing $($target.Name): $($target.Path)"
        continue
    }

    if ($PSCmdlet.ShouldProcess($target.Path, "Remove $($target.Name)")) {
        try {
            Remove-Item -LiteralPath $target.Path -Recurse -Force
            Write-Host "Removed $($target.Name): $($target.Path)"
            $removedTargets[$target.Path] = $true
        } catch {
            throw "Could not remove $($target.Name) '$($target.Path)'. No later targets were removed. Error: $($_.Exception.Message)"
        }
    }
}

if ((Test-Path -LiteralPath $serviceInstallDirectory -PathType Container) -and
    ($null -eq (Get-ChildItem -LiteralPath $serviceInstallDirectory -Force | Select-Object -First 1)) -and
    $PSCmdlet.ShouldProcess($serviceInstallDirectory, 'Remove empty K3s install directory')) {
    Remove-Item -LiteralPath $serviceInstallDirectory -Force
    Write-Host "Removed empty K3s install directory: $serviceInstallDirectory"
}

Write-Host 'K3s uninstall actions finished (or were previewed or skipped). The cluster-side Kubernetes Node object is not removed.'