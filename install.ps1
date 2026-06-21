# togo CLI installer (Windows, PowerShell).
#
#   irm https://raw.githubusercontent.com/togo-framework/cli/main/install.ps1 | iex
#
# Installs the `togo` binary. Prefers a prebuilt release asset; falls back to
# `go install` when Go is available.
$ErrorActionPreference = "Stop"

$Repo        = "togo-framework/cli"
$Bin         = "togo.exe"
$InstallPath = "github.com/togo-framework/cli/cmd/togo@latest"

function Info($m) { Write-Host "-> $m" -ForegroundColor Cyan }
function Ok($m)   { Write-Host "OK $m" -ForegroundColor Green }
function Err($m)  { Write-Host "x  $m" -ForegroundColor Red }

$arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { $arch = "arm64" }
Info "Detected windows/$arch"

function Try-Release {
  try {
    $rel = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
    $tag = $rel.tag_name
    if (-not $tag) { return $false }
    $asset = "togo_windows_${arch}.zip"
    $url   = "https://github.com/$Repo/releases/download/$tag/$asset"
    Info "Downloading $asset ($tag)"
    $tmp = New-Item -ItemType Directory -Path ([System.IO.Path]::GetTempPath() + [guid]::NewGuid())
    $zip = Join-Path $tmp $asset
    Invoke-WebRequest $url -OutFile $zip
    Expand-Archive $zip -DestinationPath $tmp -Force
    $dest = Join-Path $env:LOCALAPPDATA "togo\bin"
    New-Item -ItemType Directory -Force -Path $dest | Out-Null
    Move-Item -Force (Join-Path $tmp $Bin) (Join-Path $dest $Bin)
    Ok "Installed to $dest\$Bin"
    Add-ToPath $dest
    return $true
  } catch { return $false }
}

function Try-Go {
  if (-not (Get-Command go -ErrorAction SilentlyContinue)) { return $false }
  Info "Installing via go install ($InstallPath)"
  go install $InstallPath
  $gobin = (& go env GOBIN); if (-not $gobin) { $gobin = (Join-Path (& go env GOPATH) "bin") }
  Ok "Installed to $gobin\$Bin"
  Add-ToPath $gobin
  return $true
}

function Add-ToPath($dir) {
  $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
  if ($userPath -notlike "*$dir*") {
    [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
    Info "Added $dir to your PATH (restart your shell)"
  }
}

if (-not (Try-Release)) {
  if (-not (Try-Go)) {
    Err "Could not install: no release asset for windows/$arch and Go is not installed."
    Err "Install Go (https://go.dev/dl) or grab a binary from https://github.com/$Repo/releases"
    exit 1
  }
}
