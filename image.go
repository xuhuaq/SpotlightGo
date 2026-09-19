// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func (a *App) renderWallpaper(record ImageRecord) (string, error) {
	sourcePath := a.archivePath(&record)
	input, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	source, _, err := image.Decode(input)
	input.Close()
	if err != nil {
		return "", err
	}
	bounds := source.Bounds()
	canvas := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(canvas, canvas.Bounds(), source, bounds.Min, draw.Src)

	if a.shouldOverlayTitle(record) {
		safeBounds := safeBoundsForMonitors(canvas.Bounds(), monitorDimensions())
		if err := drawTitle(canvas, record.Title, safeBounds); err != nil {
			logError("叠加图片文字失败，继续使用无文字图片", err)
		}
	}
	outputPath := filepath.Join(a.root, "generated", record.ID[:20]+".jpg")
	output, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	err = jpeg.Encode(output, canvas, &jpeg.Options{Quality: a.config.JPEGQuality})
	closeErr := output.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	return outputPath, nil
}

func (a *App) shouldOverlayTitle(record ImageRecord) bool {
	return record.Source != "unsplash" && a.config.OverlayText && strings.TrimSpace(record.Title) != ""
}

func drawTitle(dst *image.NRGBA, title string, safeBounds image.Rectangle) error {
	fontData, err := loadSystemFont()
	if err != nil {
		return err
	}
	collection, err := opentype.ParseCollection(fontData)
	if err != nil {
		return err
	}
	parsed, err := collection.Font(0)
	if err != nil {
		return err
	}
	width := dst.Bounds().Dx()
	margin := clamp(width/192, 10, 32)
	availableWidth := safeBounds.Dx() - 2*margin
	if availableWidth <= 0 {
		return fmt.Errorf("显示器共同可见区域过窄")
	}

	fontSize := float64(clamp(width/120, 18, 42))
	var titleFace font.Face
	for fontSize >= 12 {
		titleFace, err = opentype.NewFace(parsed, &opentype.FaceOptions{Size: fontSize, DPI: 96, Hinting: font.HintingFull})
		if err != nil {
			return err
		}
		if font.MeasureString(titleFace, title).Ceil() <= availableWidth {
			break
		}
		closeFontFace(titleFace)
		titleFace = nil
		fontSize -= 2
	}
	if titleFace == nil {
		return fmt.Errorf("标题过长，无法在所有显示器中完整显示")
	}
	defer closeFontFace(titleFace)

	titleWidth := font.MeasureString(titleFace, title).Ceil()
	x := safeBounds.Max.X - margin - titleWidth
	y := safeBounds.Min.Y + margin + titleFace.Metrics().Ascent.Ceil()
	white := image.NewUniform(color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	drawText(dst, titleFace, white, x, y, title)
	return nil
}

func closeFontFace(face font.Face) {
	if closer, ok := face.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func drawText(dst draw.Image, face font.Face, src image.Image, x, baseline int, text string) {
	d := &font.Drawer{Dst: dst, Src: src, Face: face, Dot: fixed.P(x, baseline)}
	d.DrawString(text)
}

func loadSystemFont() ([]byte, error) {
	windowsDir := os.Getenv("WINDIR")
	if windowsDir == "" {
		windowsDir = `C:\Windows`
	}
	for _, name := range []string{"msyh.ttc", "msyhbd.ttc", "segoeui.ttf", "arial.ttf"} {
		data, err := os.ReadFile(filepath.Join(windowsDir, "Fonts", name))
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("没有找到可用的系统字体")
}

// safeBoundsForMonitors returns the source-image area that remains visible on
// every monitor when Windows uses the centered "Fill" wallpaper mode.
func safeBoundsForMonitors(bounds image.Rectangle, monitors []image.Point) image.Rectangle {
	width, height := bounds.Dx(), bounds.Dy()
	safe := image.Rect(0, 0, width, height)
	for _, monitor := range monitors {
		if monitor.X <= 0 || monitor.Y <= 0 {
			continue
		}
		if int64(monitor.X)*int64(height) < int64(monitor.Y)*int64(width) {
			visibleWidth := int(int64(monitor.X) * int64(height) / int64(monitor.Y))
			crop := (width - visibleWidth) / 2
			if crop > safe.Min.X {
				safe.Min.X = crop
			}
			if width-crop < safe.Max.X {
				safe.Max.X = width - crop
			}
		} else {
			visibleHeight := int(int64(monitor.Y) * int64(width) / int64(monitor.X))
			crop := (height - visibleHeight) / 2
			if crop > safe.Min.Y {
				safe.Min.Y = crop
			}
			if height-crop < safe.Max.Y {
				safe.Max.Y = height - crop
			}
		}
	}
	return safe.Add(bounds.Min)
}

func clamp(value, minimum, maximum int) int {
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
