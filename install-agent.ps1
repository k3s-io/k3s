[CmdletBinding()]
param(
    [ValidateSet("Install", "Uninstall")]
    [string]$Action = "Install",
    [string]$K3sPath,
    [string]$ConfigFile
)

$ErrorActionPreference = "Stop"

$serviceName = "k3s-agent"
$installDirectory = Join-Path $env:ProgramFiles "K3s"
$shawlVersion = "1.9.0"
$shawlArchiveUrl = "https://github.com/mtkennerly/shawl/releases/download/v$shawlVersion/shawl-v$shawlVersion-win64.zip"
$shawlArchiveSha256 = "f883c5d09c9beae2efaeabd8513e7d3f57cd1d0864cec3df4f4a7b6ee904351c"
$shawlLicenseUrl = "https://github.com/mtkennerly/shawl/releases/download/v$shawlVersion/shawl-v$shawlVersion-legal.zip"
$shawlLicenseSha256 = "a2a1743a64b79922062906eb32cdaf23db131769f8cabcc98e5d57bbf66eb0b0"

function Test-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-ServiceByName {
    return Get-Service -Name $serviceName -ErrorAction SilentlyContinue
}

function Set-RestrictedDirectoryAcl {
    param([string]$Path)

    & icacls.exe $Path /inheritance:r /grant:r "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F"
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to restrict access to '$Path' (icacls exit code $LASTEXITCODE)."
    }

    # Existing children must re-inherit from the directory; applying /T with
    # /inheritance:r would leave them with empty ACLs.
    & icacls.exe (Join-Path $Path "*") /reset /T /C | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Failed to reset inherited access on '$Path' (icacls exit code $LASTEXITCODE)."
    }
}

function Stop-K3sService {
    $service = Get-ServiceByName
    if ($service.Status -eq [System.ServiceProcess.ServiceControllerStatus]::Stopped) {
        return
    }
    if ($service.Status -ne [System.ServiceProcess.ServiceControllerStatus]::StopPending) {
        Stop-Service -Name $serviceName -Force
    }
    $service.WaitForStatus(
        [System.ServiceProcess.ServiceControllerStatus]::Stopped,
        [TimeSpan]::FromSeconds(120)
    )
}

function Save-VerifiedArchive {
    param(
        [string]$Url,
        [string]$Sha256,
        [string]$DestinationPath
    )

    Invoke-WebRequest -Uri $Url -OutFile $DestinationPath -UseBasicParsing
    $actualHash = (Get-FileHash -LiteralPath $DestinationPath -Algorithm SHA256).Hash
    if ($actualHash -ne $Sha256) {
        throw "SHA-256 verification failed for '$Url'."
    }
}

function Install-Shawl {
    $temporaryDirectory = Join-Path ([IO.Path]::GetTempPath()) ([Guid]::NewGuid().ToString("N"))
    New-Item -ItemType Directory -Path $temporaryDirectory | Out-Null
    try {
        if ($PSVersionTable.PSVersion.Major -lt 6) {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        }

        $binaryArchive = Join-Path $temporaryDirectory "shawl.zip"
        $licenseArchive = Join-Path $temporaryDirectory "shawl-legal.zip"
        Save-VerifiedArchive -Url $shawlArchiveUrl -Sha256 $shawlArchiveSha256 -DestinationPath $binaryArchive
        Save-VerifiedArchive -Url $shawlLicenseUrl -Sha256 $shawlLicenseSha256 -DestinationPath $licenseArchive

        $extractPath = Join-Path $temporaryDirectory "extracted"
        Expand-Archive -LiteralPath $binaryArchive -DestinationPath $extractPath
        Expand-Archive -LiteralPath $licenseArchive -DestinationPath $extractPath

        Copy-Item -LiteralPath (Join-Path $extractPath "shawl.exe") -Destination (Join-Path $installDirectory "shawl.exe") -Force
        Copy-Item -LiteralPath (Join-Path $extractPath "shawl-v$shawlVersion-legal.txt") -Destination (Join-Path $installDirectory "shawl-LICENSE.txt") -Force
    }
    finally {
        Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force
    }
}

$nativeArchitecture = $env:PROCESSOR_ARCHITECTURE
if ($env:PROCESSOR_ARCHITEW6432) {
    $nativeArchitecture = $env:PROCESSOR_ARCHITEW6432
}
if ($nativeArchitecture -ne "AMD64") {
    throw "The Windows K3s agent service installer currently supports AMD64 Windows only."
}
if (-not [Environment]::Is64BitProcess) {
    throw "Run this script from 64-bit PowerShell."
}
if (-not (Test-Administrator)) {
    throw "Run this script from an elevated PowerShell session."
}

$installedK3sPath = Join-Path $installDirectory "k3s.exe"
$shawlPath = Join-Path $installDirectory "shawl.exe"

switch ($Action) {
    "Install" {
        if ([string]::IsNullOrWhiteSpace($K3sPath) -or -not (Test-Path -LiteralPath $K3sPath -PathType Leaf)) {
            throw "Specify an existing Windows k3s.exe with -K3sPath."
        }
        $sourceK3sPath = (Resolve-Path -LiteralPath $K3sPath).Path
        if ([string]::IsNullOrWhiteSpace($ConfigFile) -or -not (Test-Path -LiteralPath $ConfigFile -PathType Leaf)) {
            throw "Specify an existing agent configuration file with -ConfigFile."
        }
        $resolvedConfigFile = (Resolve-Path -LiteralPath $ConfigFile).Path
        if ($null -ne (Get-ServiceByName)) {
            throw "Service '$serviceName' already exists. Use -Action Uninstall before installing again."
        }

        New-Item -ItemType Directory -Path $installDirectory -Force | Out-Null
        $serviceDataDirectory = Join-Path $env:ProgramData "k3s"
        $dataDirectory = Join-Path $serviceDataDirectory "data"
        $configPath = Join-Path $serviceDataDirectory "config.yaml"
        $logDirectory = Join-Path $serviceDataDirectory "logs"
        New-Item -ItemType Directory -Path $dataDirectory, $logDirectory -Force | Out-Null

        Set-RestrictedDirectoryAcl -Path $installDirectory
        Set-RestrictedDirectoryAcl -Path $serviceDataDirectory
        Install-Shawl
        Copy-Item -LiteralPath $resolvedConfigFile -Destination $configPath -Force

        if (-not [string]::Equals($sourceK3sPath, $installedK3sPath, [StringComparison]::OrdinalIgnoreCase)) {
            Copy-Item -LiteralPath $sourceK3sPath -Destination $installedK3sPath -Force
        }

        $shawlArguments = @(
            "add",
            "--name", $serviceName,
            "--cwd", $installDirectory,
            "--log-dir", $logDirectory,
            "--log-rotate", "daily",
            "--log-retain", "7",
            "--stop-timeout", "120000",
            "--restart-if-not", "0",
            "--restart-delay", "5000",
            "--kill-process-tree"
        )
        & $shawlPath @shawlArguments -- $installedK3sPath agent --config $configPath --data-dir $dataDirectory
        if ($LASTEXITCODE -ne 0) {
            throw "Shawl failed to install service '$serviceName' (exit code $LASTEXITCODE)."
        }

        Set-Service -Name $serviceName -StartupType Automatic
        Start-Service -Name $serviceName
        (Get-ServiceByName).WaitForStatus(
            [System.ServiceProcess.ServiceControllerStatus]::Running,
            [TimeSpan]::FromSeconds(120)
        )
        # A K3s agent with a bad config exits shortly after starting, so confirm it stays up.
        Start-Sleep -Seconds 10
        $service = Get-ServiceByName
        if ($service.Status -ne [System.ServiceProcess.ServiceControllerStatus]::Running) {
            throw "Service '$serviceName' stopped during startup. Check the logs in '$logDirectory'."
        }

        Write-Output "Installed and started Windows agent service '$serviceName'."
    }
    "Uninstall" {
        $service = Get-ServiceByName
        if ($null -eq $service) {
            Write-Output "Windows agent service '$serviceName' is not installed."
            break
        }

        Stop-K3sService
        & sc.exe delete $serviceName
        if ($LASTEXITCODE -ne 0) {
            throw "Windows failed to remove service '$serviceName' (exit code $LASTEXITCODE)."
        }

        $deadline = [DateTime]::UtcNow.AddSeconds(30)
        while ($null -ne (Get-ServiceByName) -and [DateTime]::UtcNow -lt $deadline) {
            Start-Sleep -Seconds 1
        }
        if ($null -ne (Get-ServiceByName)) {
            throw "Service '$serviceName' is marked for deletion but remains registered. Retry after Windows releases it."
        }
        Write-Output "Removed Windows agent service '$serviceName'. K3s data, configuration, and installed files were left intact."
    }
}
