// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Locale               string         `json:"locale"`
	Country              string         `json:"country"`
	OverlayText          bool           `json:"overlay_text"`
	AutoDownloadHours    int            `json:"auto_download_hours"`
	CheckIntervalMinutes int            `json:"check_interval_minutes"`
	DefaultSwitchHours   int            `json:"default_switch_hours"`
	JPEGQuality          int            `json:"jpeg_quality"`
	UnsplashAccessKey    string         `json:"unsplash_access_key,omitempty"`
	UnsplashTopic        string         `json:"unsplash_topic"`
	UnsplashDownloads    int            `json:"unsplash_downloads"`
	UnsplashMinWidth     int            `json:"unsplash_min_width"`
	UnsplashMinHeight    int            `json:"unsplash_min_height"`
	RatingDisplayHours   map[string]int `json:"rating_display_hours"`
}

type ImageRecord struct {
	ID           string    `json:"id"`
	FileName     string    `json:"file_name"`
	SourceURL    string    `json:"source_url"`
	Title        string    `json:"title,omitempty"`
	Description  string    `json:"description,omitempty"`
	Copyright    string    `json:"copyright,omitempty"`
	Source       string    `json:"source,omitempty"`
	SourceID     string    `json:"source_id,omitempty"`
	SourcePage   string    `json:"source_page,omitempty"`
	Author       string    `json:"author,omitempty"`
	AuthorPage   string    `json:"author_page,omitempty"`
	Likes        int       `json:"likes,omitempty"`
	DownloadAPI  string    `json:"download_location,omitempty"`
	Favorite     bool      `json:"favorite,omitempty"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	Rating       int       `json:"rating"`
	DownloadedAt time.Time `json:"downloaded_at"`
	LastShownAt  time.Time `json:"last_shown_at,omitempty"`
	ShowCount    int       `json:"show_count"`
}

type State struct {
	Images            map[string]*ImageRecord `json:"images"`
	CurrentID         string                  `json:"current_id,omitempty"`
	NextSwitchAt      time.Time               `json:"next_switch_at,omitempty"`
	ScheduleStartedAt time.Time               `json:"schedule_started_at,omitempty"`
	ScheduleHours     int                     `json:"schedule_hours,omitempty"`
	LastDownloadCheck time.Time               `json:"last_download_check,omitempty"`
	UnsplashPage      int                     `json:"unsplash_page,omitempty"`
}

type App struct {
	root      string
	config    Config
	state     State
	mu        sync.Mutex
	workMu    sync.Mutex
	setStatus func(string)
}

func defaultConfig() Config {
	return Config{
		Locale:               "zh-CN",
		Country:              "CN",
		OverlayText:          true,
		AutoDownloadHours:    24,
		CheckIntervalMinutes: 15,
		DefaultSwitchHours:   6,
		JPEGQuality:          95,
		UnsplashTopic:        "wallpapers",
		UnsplashDownloads:    8,
		UnsplashMinWidth:     3840,
		UnsplashMinHeight:    2160,
		RatingDisplayHours: map[string]int{
			"1": 2,
			"2": 8,
			"3": 6,
			"4": 72,
			"5": 168,
		},
	}
}

func newApp(root string) (*App, error) {
	if err := os.MkdirAll(filepath.Join(root, "archive"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "metadata"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "generated"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(root, "favorites"), 0o755); err != nil {
		return nil, err
	}

	cfg := defaultConfig()
	configPath := filepath.Join(root, "config.json")
	if err := readJSON(configPath, &cfg); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	normalizeConfig(&cfg)
	if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
		if err := writeJSON(configPath, cfg); err != nil {
			return nil, err
		}
	}

	state := State{Images: make(map[string]*ImageRecord)}
	if err := readJSON(filepath.Join(root, "state.json"), &state); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if state.Images == nil {
		state.Images = make(map[string]*ImageRecord)
	}
	app := &App{root: root, config: cfg, state: state}
	if err := app.reconcileSchedule(); err != nil {
		return nil, err
	}
	return app, nil
}

func normalizeConfig(cfg *Config) {
	if strings.TrimSpace(cfg.Locale) == "" {
		cfg.Locale = "zh-CN"
	}
	if strings.TrimSpace(cfg.Country) == "" {
		parts := strings.Split(cfg.Locale, "-")
		if len(parts) == 2 {
			cfg.Country = strings.ToUpper(parts[1])
		} else {
			cfg.Country = "CN"
		}
	}
	if cfg.AutoDownloadHours < 1 {
		cfg.AutoDownloadHours = 24
	}
	if cfg.CheckIntervalMinutes < 1 {
		cfg.CheckIntervalMinutes = 15
	}
	if cfg.DefaultSwitchHours < 1 {
		cfg.DefaultSwitchHours = 6
	}
	if cfg.JPEGQuality < 70 || cfg.JPEGQuality > 100 {
		cfg.JPEGQuality = 95
	}
	if strings.TrimSpace(cfg.UnsplashTopic) == "" {
		cfg.UnsplashTopic = "wallpapers"
	}
	if cfg.UnsplashDownloads < 1 || cfg.UnsplashDownloads > 30 {
		cfg.UnsplashDownloads = 8
	}
	if cfg.UnsplashMinWidth < 1 {
		cfg.UnsplashMinWidth = 3840
	}
	if cfg.UnsplashMinHeight < 1 {
		cfg.UnsplashMinHeight = 2160
	}
	defaults := defaultConfig().RatingDisplayHours
	if cfg.RatingDisplayHours == nil {
		cfg.RatingDisplayHours = defaults
	} else {
		for key, value := range defaults {
			if cfg.RatingDisplayHours[key] < 1 {
				cfg.RatingDisplayHours[key] = value
			}
		}
	}
}

func readJSON(path string, target any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err == nil {
		return nil
	}
	// Windows cannot always replace an existing file with os.Rename.
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmp, path)
}

func (a *App) saveStateLocked() error {
	return writeJSON(filepath.Join(a.root, "state.json"), a.state)
}

func (a *App) status(message string) {
	if a.setStatus != nil {
		a.setStatus(message)
	}
}
