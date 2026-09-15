# Copyright 2026 SpotlightGo contributors.
# SPDX-License-Identifier: CDDL-1.0

param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$')]
    [string]$Version
)

$ErrorActionPreference = 'Stop'
$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$distDir = Join-Path $projectDir 'dist'
$packageName = "SpotlightGo-$Version-windows-amd64"
$packageDir = Join-Path $distDir $packageName
$zipPath = Join-Path $distDir "$packageName.zip"

$distFull = [System.IO.Path]::GetFullPath($distDir)
$packageFull = [System.IO.Path]::GetFullPath($packageDir)
$zipFull = [System.IO.Path]::GetFullPath($zipPath)
foreach ($target in $packageFull, $zipFull) {
    if (-not $target.StartsWith($distFull + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "拒绝操作发布目录以外的路径：$target"
    }
}

& (Join-Path $projectDir 'build.ps1') -Version $Version
if ($LASTEXITCODE -ne 0) { throw '发布版本编译失败。' }

if (Test-Path -LiteralPath $packageDir) {
    Remove-Item -LiteralPath $packageDir -Recurse -Force
}
if (Test-Path -LiteralPath $zipPath) {
    Remove-Item -LiteralPath $zipPath -Force
}
New-Item -ItemType Directory -Force -Path $packageDir | Out-Null
Copy-Item -LiteralPath (Join-Path $projectDir 'SpotlightGo.exe') -Destination $packageDir
foreach ($file in 'README.md', 'LICENSE', 'NOTICE', 'THIRD_PARTY_NOTICES.md', 'config.example.json') {
    Copy-Item -LiteralPath (Join-Path $projectDir $file) -Destination $packageDir
}
Copy-Item -LiteralPath (Join-Path $projectDir 'licenses') -Destination $packageDir -Recurse
Compress-Archive -Path (Join-Path $packageDir '*') -DestinationPath $zipPath

$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath).Hash.ToLowerInvariant()
$sumPath = Join-Path $distDir 'SHA256SUMS.txt'
"$hash  $packageName.zip" | Set-Content -Encoding ascii -LiteralPath $sumPath
Write-Host "发布包：$zipPath"
Write-Host "校验值：$hash"
