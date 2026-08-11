param(
    [Parameter(Position = 0)]
    [string]$Version = $env:GOSVC_VERSION,
    [string]$Repository = $(if ($env:GOSVC_REPOSITORY) { $env:GOSVC_REPOSITORY } else { '__GOSVC_REPOSITORY__' }),
    [string]$InstallDir = $env:GOSVC_INSTALL_DIR,
    [string]$ReleaseBaseUrl = $env:GOSVC_RELEASE_BASE_URL
)

$ErrorActionPreference = 'Stop'
if (-not $Version) { throw 'Version is required. Example: .\install.ps1 1.0.0 -Repository ailtonmacedo/gosvc' }
if (-not $Repository) { throw 'Repository is required. Set -Repository ailtonmacedo/gosvc or GOSVC_REPOSITORY.' }
if (-not $InstallDir) { $InstallDir = Join-Path $HOME '.local\bin' }

$Version = $Version.TrimStart('v')
$Architecture = switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
    'X64' { 'amd64' }
    'Arm64' { 'arm64' }
    default { throw "Unsupported architecture: $($_)" }
}
$Asset = "gosvc_${Version}_windows_${Architecture}.zip"
$BaseUrl = if ($ReleaseBaseUrl) { $ReleaseBaseUrl } else { "https://github.com/${Repository}/releases/download/v${Version}" }
$Temporary = Join-Path ([System.IO.Path]::GetTempPath()) ("gosvc-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $Temporary | Out-Null
try {
    Invoke-WebRequest "${BaseUrl}/${Asset}" -OutFile (Join-Path $Temporary $Asset)
    Invoke-WebRequest "${BaseUrl}/checksums.txt" -OutFile (Join-Path $Temporary 'checksums.txt')
    $Line = Get-Content (Join-Path $Temporary 'checksums.txt') | Where-Object { $_ -match "\s${([regex]::Escape($Asset))}$" }
    if (-not $Line) { throw "Checksum for $Asset not found" }
    $Expected = ($Line -split '\s+')[0]
    $Actual = (Get-FileHash (Join-Path $Temporary $Asset) -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($Expected.ToLowerInvariant() -ne $Actual) { throw "Checksum verification failed for $Asset" }

    Expand-Archive (Join-Path $Temporary $Asset) -DestinationPath $Temporary
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
    Copy-Item (Join-Path $Temporary "gosvc_${Version}_windows_${Architecture}\gosvc.exe") (Join-Path $InstallDir 'gosvc.exe') -Force
    Write-Host "gosvc $Version installed at $(Join-Path $InstallDir 'gosvc.exe')"
}
finally {
    Remove-Item -Recurse -Force $Temporary -ErrorAction SilentlyContinue
}
