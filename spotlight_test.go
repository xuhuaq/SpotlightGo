// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	if app.displayDuration(3) != 6*time.Hour {
		t.Fatalf("default 3-star duration = %v", app.displayDuration(3))
	}
	if app.displayDuration(5) != 7*24*time.Hour {
		t.Fatalf("5-star duration = %v", app.displayDuration(5))
	}
	if app.displayDuration(1) >= app.displayDuration(3) {
		t.Fatal("low-rated image should stay for less time")
	}
}

func TestLegacyScheduleUsesConfiguredDefaultWithoutRestartReset(t *testing.T) {
	root := t.TempDir()
	app, err := newApp(root)
	if err != nil {
		t.Fatal(err)
	}
	shownAt := time.Now().Add(-2 * time.Hour).Truncate(time.Second)
	app.state.Images["legacy"] = &ImageRecord{ID: "legacy", FileName: "legacy.jpg", Rating: 3, LastShownAt: shownAt}
	app.state.CurrentID = "legacy"
	app.state.NextSwitchAt = shownAt.Add(24 * time.Hour)
	if err := app.saveStateLocked(); err != nil {
		t.Fatal(err)
	}

	migrated, err := newApp(root)
	if err != nil {
		t.Fatal(err)
	}
	want := shownAt.Add(6 * time.Hour)
	if !migrated.state.NextSwitchAt.Equal(want) {
		t.Fatalf("migrated next switch = %v, want %v", migrated.state.NextSwitchAt, want)
	}
	if migrated.state.ScheduleHours != 6 || !migrated.state.ScheduleStartedAt.Equal(shownAt) {
		t.Fatalf("schedule metadata not migrated: %+v", migrated.state)
	}
}

func TestFavoriteCurrentCopiesOriginalAndMarksFiveStars(t *testing.T) {
	root := t.TempDir()
	app, err := newApp(root)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	fileName := id[:20] + ".jpg"
	original := []byte("original image bytes")
	if err := os.WriteFile(filepath.Join(root, "archive", fileName), original, 0o644); err != nil {
		t.Fatal(err)
	}
	app.state.Images[id] = &ImageRecord{ID: id, FileName: fileName, Rating: 3}
	app.state.CurrentID = id
	if err := app.favoriteCurrent(); err != nil {
		t.Fatal(err)
	}
	if !app.state.Images[id].Favorite || app.state.Images[id].Rating != 5 {
		t.Fatalf("favorite state not recorded: %+v", app.state.Images[id])
	}
	got, err := os.ReadFile(filepath.Join(root, "favorites", fileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(original) {
		t.Fatal("favorite copy differs from archived original")
	}
}

func TestDeleteOnlyImageRemovesAllCopies(t *testing.T) {
	root := t.TempDir()
	app, err := newApp(root)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("b", 64)
	fileName := id[:20] + ".jpg"
	paths := []string{
		filepath.Join(root, "archive", fileName),
		filepath.Join(root, "favorites", fileName),
		filepath.Join(root, "generated", fileName),
		filepath.Join(root, "metadata", id[:20]+".json"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app.state.Images[id] = &ImageRecord{ID: id, FileName: fileName, Rating: 3}
	app.state.CurrentID = id
	if err := app.deleteCurrentAndSwitch(); err != nil {
		t.Fatal(err)
	}
	if len(app.state.Images) != 0 || app.state.CurrentID != "" {
		t.Fatalf("deleted image remains in state: %+v", app.state)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("deleted file still exists: %s", path)
		}
	}
}

func TestUnsplashJPEGURLPreservesTrackingParameters(t *testing.T) {
	got, err := unsplashJPEGURL("https://images.unsplash.com/photo-1?ixid=tracking-value&w=100")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ixid=tracking-value", "fm=jpg", "q=95", "w=100"} {
		if !strings.Contains(got, want) {
			t.Fatalf("URL %q does not contain %q", got, want)
		}
	}
}

func TestUnsplashReferralURL(t *testing.T) {
	got, err := addReferralParameters("https://unsplash.com/@photographer?existing=yes")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"existing=yes", "utm_source=spotlightgo", "utm_medium=referral"} {
		if !strings.Contains(got, want) {
			t.Fatalf("URL %q does not contain %q", got, want)
		}
	}
}

func TestUnsplashTrackingRejectsOtherHosts(t *testing.T) {
	app := &App{config: defaultConfig()}
	err := app.trackUnsplashDownload(context.Background(), &http.Client{}, "https://example.com/photos/abc/download")
	if err == nil || !strings.Contains(err.Error(), "非 Unsplash API") {
		t.Fatalf("unexpected tracking validation result: %v", err)
	}
}

func TestChooseImageCompletesCycleBeforeRepeating(t *testing.T) {
	now := time.Now()
	images := map[string]*ImageRecord{
		"a": {ID: "a", Rating: 3, ShowCount: 1, LastShownAt: now},
		"b": {ID: "b", Rating: 3},
		"c": {ID: "c", Rating: 3},
	}

	first, err := chooseImage(images, "a")
	if err != nil {
		t.Fatal(err)
	}
	images[first.ID].ShowCount++
	images[first.ID].LastShownAt = now.Add(time.Minute)

	second, err := chooseImage(images, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID == first.ID || second.ID == "a" {
		t.Fatalf("selection repeated before every image had a turn: %q then %q", first.ID, second.ID)
	}
}

func TestChooseImageRatingDoesNotOverrideFairRotation(t *testing.T) {
	images := map[string]*ImageRecord{
		"new-low-rated":        {ID: "new-low-rated", Rating: 1},
		"often-shown-favorite": {ID: "often-shown-favorite", Rating: 5, ShowCount: 8, LastShownAt: time.Now()},
	}
	record, err := chooseImage(images, "")
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "new-low-rated" {
		t.Fatalf("rating overrode fair rotation: chose %q", record.ID)
	}
}

func TestTrayIconIsICO(t *testing.T) {
	icon := trayIcon()
	if len(icon) < 22 || icon[2] != 1 || icon[4] != 9 {
		t.Fatalf("invalid ICO header: %v", icon[:6])
	}
	fileIcon, err := os.ReadFile(filepath.Join("assets", "spotlightgo.ico"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(icon, fileIcon) {
		t.Fatal("tray icon differs from the application icon asset")
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

func TestUnsplashNeverOverlaysTitle(t *testing.T) {
	app := &App{config: defaultConfig()}
	if app.shouldOverlayTitle(ImageRecord{Source: "unsplash", Title: "Unsplash title"}) {
		t.Fatal("Unsplash image must not receive a title overlay")
	}
	if !app.shouldOverlayTitle(ImageRecord{Source: "spotlight", Title: "Spotlight title"}) {
		t.Fatal("Spotlight title overlay should remain enabled")
	}
}
