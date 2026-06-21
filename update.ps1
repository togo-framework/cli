# Update the togo CLI to the latest version (Windows, PowerShell).
#
#   irm https://raw.githubusercontent.com/togo-framework/cli/main/update.ps1 | iex
$ErrorActionPreference = "Stop"
Write-Host "-> Updating togo..." -ForegroundColor Cyan
Invoke-RestMethod https://raw.githubusercontent.com/togo-framework/cli/main/install.ps1 | Invoke-Expression
Write-Host "OK togo is up to date" -ForegroundColor Green
