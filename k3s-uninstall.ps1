[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'High')]
param(
    [string]$DataDir = $(
        if (-not [string]::IsNullOrWhiteSpace($env:K3S_DATA_DIR)) {
            $env:K3S_DATA_DIR
        } else {
            Join-Path $env:SystemDrive 'var\lib\rancher\k3s'
        }
    ),
    [string]$ConfigDir = (Join-Path $env:SystemDrive 'etc\rancher\k3s'),
    [string]$BinaryPath
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

$runningK3s = Get-Process -Name 'k3s' -ErrorAction SilentlyContinue
if ($runningK3s) {
    $processIds = ($runningK3s | ForEach-Object { $_.Id }) -join ', '
    throw "K3s is still running (process ID(s): $processIds). Stop it and rerun this script."
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

    return $fullPath
}

$targets = @(
    [pscustomobject]@{
        Name = 'K3s data directory'
        Path = Resolve-RemovalPath -Path $DataDir -Name 'the K3s data directory'
    },
    [pscustomobject]@{
        Name = 'K3s configuration directory'
        Path = Resolve-RemovalPath -Path $ConfigDir -Name 'the K3s configuration directory'
    }
)

if (-not [string]::IsNullOrWhiteSpace($BinaryPath)) {
    $targets += [pscustomobject]@{
        Name = 'K3s executable'
        Path = Resolve-RemovalPath -Path $BinaryPath -Name 'the K3s executable'
    }
}

foreach ($target in $targets) {
    if (-not (Test-Path -LiteralPath $target.Path)) {
        Write-Host "Skipping missing $($target.Name): $($target.Path)"
        continue
    }

    if ($PSCmdlet.ShouldProcess($target.Path, "Remove $($target.Name)")) {
        try {
            Remove-Item -LiteralPath $target.Path -Recurse -Force
            Write-Host "Removed $($target.Name): $($target.Path)"
        } catch {
            if ($target.Name -eq 'K3s data directory') {
                throw "Could not completely remove the K3s data directory '$($target.Path)'. Windows container snapshots may be HCS-managed and need snapshotter-specific cleanup. No later targets were removed. Error: $($_.Exception.Message)"
            }

            throw "Could not remove $($target.Name) '$($target.Path)'. Error: $($_.Exception.Message)"
        }
    }
}

Write-Host 'K3s cleanup actions finished (or were previewed or skipped). This repository does not register a Windows service; externally registered services are left untouched.'