# Copyright 2026 SpotlightGo contributors.
# SPDX-License-Identifier: CDDL-1.0

param(
    [string]$Version = 'dev'
)

$ErrorActionPreference = 'Stop'

$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$portableGo = Join-Path $projectDir '.tools\go\bin\go.exe'
$goCommand = Get-Command go -ErrorAction SilentlyContinue

if ($goCommand) {
    $goExe = $goCommand.Source
} elseif (Test-Path -LiteralPath $portableGo) {
    $goExe = $portableGo
} else {
    throw '没有找到 Go。请先安装 Go，或运行 install-portable-go.ps1。'
}

Push-Location $projectDir
try {
    $env:GOCACHE = Join-Path $projectDir '.gocache'
    $env:GOMODCACHE = Join-Path $projectDir '.gomodcache'
    & $goExe mod download all
    & $goExe test ./...
    if ($LASTEXITCODE -ne 0) { throw '测试失败。' }
    & $goExe vet ./...
    if ($LASTEXITCODE -ne 0) { throw '静态检查失败。' }
    & $goExe build -trimpath -ldflags "-s -w -H=windowsgui -X main.version=$Version" -o SpotlightGo.exe .
    if ($LASTEXITCODE -ne 0) { throw '编译失败。' }
    Write-Host "已生成：$projectDir\SpotlightGo.exe"
} finally {
    Pop-Location
}
