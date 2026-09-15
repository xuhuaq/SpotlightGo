# Copyright 2026 SpotlightGo contributors.
# SPDX-License-Identifier: CDDL-1.0

$ErrorActionPreference = 'Stop'

$version = '5.2.0'
$expectedHash = 'b471ebf1b7f20f4a89150264ed9a008a2a5bfd247f3c6d1184a75bb59ca08f5d'
$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$toolsDir = [System.IO.Path]::GetFullPath((Join-Path $projectDir '.tools'))
$installDir = [System.IO.Path]::GetFullPath((Join-Path $toolsDir 'upx'))
$tempDir = [System.IO.Path]::GetFullPath((Join-Path $toolsDir 'upx-install-temp'))
$zipPath = Join-Path $tempDir "upx-$version-win64.zip"
$downloadURL = "https://github.com/upx/upx/releases/download/v$version/upx-$version-win64.zip"

foreach ($target in $installDir, $tempDir) {
    if (-not $target.StartsWith($toolsDir + [System.IO.Path]::DirectorySeparatorChar, [System.StringComparison]::OrdinalIgnoreCase)) {
        throw "拒绝操作工具目录以外的路径：$target"
    }
}

if (Test-Path -LiteralPath (Join-Path $installDir 'upx.exe')) {
    Write-Host "UPX 已安装：$installDir\upx.exe"
    exit 0
}

if (Test-Path -LiteralPath $tempDir) {
    Remove-Item -LiteralPath $tempDir -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $tempDir | Out-Null

try {
    Invoke-WebRequest -UseBasicParsing -Uri $downloadURL -OutFile $zipPath
    $actualHash = (Get-FileHash -Algorithm SHA256 -LiteralPath $zipPath).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
        throw "UPX 校验失败：$actualHash"
    }
    Expand-Archive -LiteralPath $zipPath -DestinationPath $tempDir -Force
    $upxExe = Get-ChildItem -LiteralPath $tempDir -Filter upx.exe -Recurse | Select-Object -First 1
    if (-not $upxExe) {
        throw 'UPX 发布包中没有 upx.exe。'
    }
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Copy-Item -LiteralPath $upxExe.FullName -Destination (Join-Path $installDir 'upx.exe')
    Write-Host "已安装：$installDir\upx.exe"
} finally {
    if (Test-Path -LiteralPath $tempDir) {
        Remove-Item -LiteralPath $tempDir -Recurse -Force
    }
}
