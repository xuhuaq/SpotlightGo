# Copyright 2026 SpotlightGo contributors.
# SPDX-License-Identifier: CDDL-1.0

param(
    [string]$Source = 'assets\spotlightgo-icon-source.png',
    [string]$Output = 'assets\spotlightgo.ico'
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing

$projectDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$sourcePath = [System.IO.Path]::GetFullPath((Join-Path $projectDir $Source))
$outputPath = [System.IO.Path]::GetFullPath((Join-Path $projectDir $Output))
$previewPath = [System.IO.Path]::GetFullPath((Join-Path $projectDir 'assets\spotlightgo-icon.png'))
$sizes = @(16, 20, 24, 32, 40, 48, 64, 128, 256)

$sourceImage = [System.Drawing.Bitmap]::FromFile($sourcePath)
try {
    $frames = [System.Collections.Generic.List[byte[]]]::new()
    foreach ($size in $sizes) {
        $bitmap = [System.Drawing.Bitmap]::new($size, $size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
        try {
            $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
            try {
                $graphics.Clear([System.Drawing.Color]::Transparent)
                $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
                $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
                $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
                $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
                $padding = [Math]::Max(1, [int]($size * 0.06))
                $available = $size - 2 * $padding
                $scale = [Math]::Min($available / $sourceImage.Width, $available / $sourceImage.Height)
                $width = [Math]::Max(1, [int][Math]::Round($sourceImage.Width * $scale))
                $height = [Math]::Max(1, [int][Math]::Round($sourceImage.Height * $scale))
                $x = [int](($size - $width) / 2)
                $y = [int](($size - $height) / 2)
                $graphics.DrawImage($sourceImage, $x, $y, $width, $height)
            } finally {
                $graphics.Dispose()
            }
            $stream = [System.IO.MemoryStream]::new()
            try {
                $bitmap.Save($stream, [System.Drawing.Imaging.ImageFormat]::Png)
                $frames.Add($stream.ToArray())
            } finally {
                $stream.Dispose()
            }
            if ($size -eq 256) {
                $bitmap.Save($previewPath, [System.Drawing.Imaging.ImageFormat]::Png)
            }
        } finally {
            $bitmap.Dispose()
        }
    }
} finally {
    $sourceImage.Dispose()
}

$file = [System.IO.File]::Create($outputPath)
$writer = [System.IO.BinaryWriter]::new($file)
try {
    $writer.Write([uint16]0)
    $writer.Write([uint16]1)
    $writer.Write([uint16]$frames.Count)
    $offset = 6 + 16 * $frames.Count
    for ($index = 0; $index -lt $frames.Count; $index++) {
        $size = $sizes[$index]
        $writer.Write([byte]$(if ($size -eq 256) { 0 } else { $size }))
        $writer.Write([byte]$(if ($size -eq 256) { 0 } else { $size }))
        $writer.Write([byte]0)
        $writer.Write([byte]0)
        $writer.Write([uint16]1)
        $writer.Write([uint16]32)
        $writer.Write([uint32]$frames[$index].Length)
        $writer.Write([uint32]$offset)
        $offset += $frames[$index].Length
    }
    foreach ($frame in $frames) {
        $writer.Write($frame)
    }
} finally {
    $writer.Dispose()
    $file.Dispose()
}

Write-Host "已生成：$outputPath"
