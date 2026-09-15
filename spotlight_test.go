// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseV4Fixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "v4.json"))
	if err != nil {
		t.Fatal(err)
	}
	items, err := parseAPIResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].URL == "" || items[0].Title == "" || items[0].Description == "" {
		t.Fatalf("first item was not fully parsed: %+v", items[0])
	}
}

func TestStateRoundTrip(t *testing.T) {
	root := t.TempDir()
	app, err := newApp(root)
	if err != nil {
		t.Fatal(err)
	}
	app.state.Images["abc"] = &ImageRecord{ID: "abc", FileName: "abc.jpg", Rating: 4}
	app.state.CurrentID = "abc"
	if err := app.saveStateLocked(); err != nil {
		t.Fatal(err)
	}
	loaded, err := newApp(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.state.Images["abc"].Rating != 4 || loaded.state.CurrentID != "abc" {
		t.Fatalf("state did not round-trip: %+v", loaded.state)
	}
}

func TestRatingControlsDuration(t *testing.T) {
	app := &App{config: defaultConfig()}
	if app.displayDuration(5) != 7*24*time.Hour {
		t.Fatalf("5-star duration = %v", app.displayDuration(5))
	}
	if app.displayDuration(1) >= app.displayDuration(3) {
		t.Fatal("low-rated image should stay for less time")
	}
}

func TestTrayIconIsICO(t *testing.T) {
	icon := trayIcon()
	if len(icon) < 22 || icon[2] != 1 || icon[4] != 1 {
		t.Fatalf("invalid ICO header: %v", icon[:6])
	}
}

func TestRenderedWallpaperKeepsOriginalDimensions(t *testing.T) {
	root := t.TempDir()
	app, err := newApp(root)
	if err != nil {
		t.Fatal(err)
	}
	const id = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	record := ImageRecord{ID: id, FileName: "source.jpg", Title: "原分辨率测试", Rating: 3}
	source, err := os.Create(filepath.Join(root, "archive", record.FileName))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 800, 450))
	for y := 0; y < 450; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 90, A: 255})
		}
	}
	if err := jpeg.Encode(source, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if err := source.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := app.renderWallpaper(record)
	if err != nil {
		t.Fatal(err)
	}
	result, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	cfg, _, err := image.DecodeConfig(result)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 800 || cfg.Height != 450 {
		t.Fatalf("render changed dimensions to %dx%d", cfg.Width, cfg.Height)
	}
}

func TestSafeBoundsAcrossDifferentMonitorRatios(t *testing.T) {
	bounds := image.Rect(0, 0, 3840, 2160)
	monitors := []image.Point{
		image.Pt(1920, 1080),
		image.Pt(1280, 1024),
	}
	safe := safeBoundsForMonitors(bounds, monitors)
	if safe.Min.X != 570 || safe.Max.X != 3270 || safe.Min.Y != 0 || safe.Max.Y != 2160 {
		t.Fatalf("unexpected safe bounds: %v", safe)
	}
}

func TestWindowsMonitorEnumeration(t *testing.T) {
	monitors := monitorDimensions()
	if len(monitors) == 0 {
		t.Skip("Windows session exposes no display device")
	}
	t.Logf("detected monitor dimensions: %v", monitors)
}

func TestTitleOverlayIsPlainAndAtUpperRight(t *testing.T) {
	canvas := image.NewNRGBA(image.Rect(0, 0, 800, 450))
	background := color.NRGBA{R: 20, G: 40, B: 60, A: 255}
	for y := 0; y < 450; y++ {
		for x := 0; x < 800; x++ {
			canvas.SetNRGBA(x, y, background)
		}
	}
	if err := drawTitle(canvas, "测试标题", canvas.Bounds()); err != nil {
		t.Fatal(err)
	}
	changed := 0
	minX, minY := 800, 450
	maxX, maxY := 0, 0
	for y := 0; y < 450; y++ {
		for x := 0; x < 800; x++ {
			if canvas.NRGBAAt(x, y) != background {
				changed++
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if changed == 0 {
		t.Fatal("title drew no pixels")
	}
	if changed > 800*450/20 {
		t.Fatalf("too many pixels changed (%d), possible background panel", changed)
	}
	if minX < 400 || minY > 100 || maxY > 150 {
		t.Fatalf("title is not confined to upper right: (%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
	}
}
