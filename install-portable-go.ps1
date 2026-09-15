# Copyright 2026 SpotlightGo contributors.
# SPDX-License-Identifier: CDDL-1.0

$ErrorActionPreference = 'Stop'

$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$toolsDir = Join-Path $projectDir '.tools'
$goDir = Join-Path $toolsDir 'go'

if (Test-Path -LiteralPath (Join-Path $goDir 'bin\go.exe')) {
    Write-Host "便携 Go 已存在：$goDir"
    exit 0
}

$release = Invoke-RestMethod 'https://go.dev/dl/?mode=json' | Select-Object -First 1
$file = $release.files | Where-Object { $_.os -eq 'windows' -and $_.arch -eq 'amd64' -and $_.kind -eq 'archive' } | Select-Object -First 1
if (-not $file) { throw '无法在 Go 官方发布信息中找到 Windows amd64 压缩包。' }

New-Item -ItemType Directory -Force -Path $toolsDir | Out-Null
$zipPath = Join-Path $toolsDir $file.filename
Write-Host "下载 $($file.filename)…"
Invoke-WebRequest ("https://go.dev/dl/" + $file.filename) -OutFile $zipPath

$actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath).Hash.ToLowerInvariant()
if ($actualHash -ne $file.sha256.ToLowerInvariant()) {
    throw "Go 工具链校验失败。期望 $($file.sha256)，实际 $actualHash"
}
Expand-Archive -LiteralPath $zipPath -DestinationPath $toolsDir
Remove-Item -LiteralPath $zipPath
Write-Host "便携 Go 已安装到：$goDir"
Write-Host '现在可以运行 build.ps1。'
