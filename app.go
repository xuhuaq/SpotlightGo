// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"context"
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (a *App) switchWallpaper() error {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	return a.switchWallpaperForSourceLocked("")
}

func (a *App) switchWallpaperLocked() error {
	return a.switchWallpaperForSourceLocked("")
}

func (a *App) switchWallpaperFromSource(source string) error {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	return a.switchWallpaperForSourceLocked(storageSource(source))
}

func (a *App) switchWallpaperForSourceLocked(source string) error {
	a.status("正在选择壁纸…")

	a.mu.Lock()
	record, err := chooseImageFromSource(a.state.Images, a.state.CurrentID, source)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	outputPath, err := a.renderWallpaper(*record)
	if err != nil {
		return fmt.Errorf("生成壁纸: %w", err)
	}
	if err := setWallpaper(outputPath); err != nil {
		return err
	}
	// Splashy follows the same Unsplash rule: applying a photo as wallpaper is
	// a download-like action. Tracking errors are logged but never block use.
	if record.Source == "unsplash" && record.DownloadAPI != "" && strings.TrimSpace(a.config.UnsplashAccessKey) != "" {
		client := &http.Client{Timeout: 10 * time.Second}
		if err := a.trackUnsplashDownload(context.Background(), client, record.DownloadAPI); err != nil {
			logError("报告 Unsplash 壁纸使用失败", err)
		}
	}

	now := time.Now()
	a.mu.Lock()
	current := a.state.Images[record.ID]
	current.LastShownAt = now
	current.ShowCount++
	a.state.CurrentID = record.ID
	a.setScheduleLocked(now, current.Rating)
	err = a.saveStateLocked()
	next := a.state.NextSwitchAt
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.status(currentStatus(current, time.Until(next)))
	return nil
}

func (a *App) favoriteCurrent() error {
	a.workMu.Lock()
	defer a.workMu.Unlock()

	a.mu.Lock()
	record := a.state.Images[a.state.CurrentID]
	if record == nil {
		a.mu.Unlock()
		return fmt.Errorf("当前没有壁纸")
	}
	copyOfRecord := *record
	a.mu.Unlock()
	if err := validateStoredRecord(&copyOfRecord); err != nil {
		return err
	}

	data, err := os.ReadFile(a.archivePath(&copyOfRecord))
	if err != nil {
		return fmt.Errorf("读取收藏原图: %w", err)
	}
	favoriteRecord := copyOfRecord
	favoriteRecord.Favorite = true
	favoriteRecord.Rating = 5
	if err := os.WriteFile(filepath.Join(a.root, "favorites", favoriteRecord.FileName), data, 0o644); err != nil {
		return fmt.Errorf("保存收藏原图: %w", err)
	}
	if err := writeJSON(filepath.Join(a.root, "favorites", favoriteRecord.ID[:20]+".json"), &favoriteRecord); err != nil {
		return fmt.Errorf("保存收藏信息: %w", err)
	}

	a.mu.Lock()
	record = a.state.Images[copyOfRecord.ID]
	if record == nil {
		a.mu.Unlock()
		return fmt.Errorf("图片记录已不存在")
	}
	record.Favorite = true
	record.Rating = 5
	a.setScheduleLocked(time.Now(), record.Rating)
	if err := writeJSON(a.metadataPath(record), record); err != nil {
		a.mu.Unlock()
		return err
	}
	err = a.saveStateLocked()
	name := displayName(record)
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.status(fmt.Sprintf("已喜欢，收藏“%s”（5 星）", name))
	return nil
}

func (a *App) deleteCurrentAndSwitch() error {
	a.workMu.Lock()
	defer a.workMu.Unlock()

	a.mu.Lock()
	record := a.state.Images[a.state.CurrentID]
	if record == nil {
		a.mu.Unlock()
		return fmt.Errorf("当前没有壁纸")
	}
	copyOfRecord := *record
	if err := validateStoredRecord(&copyOfRecord); err != nil {
		a.mu.Unlock()
		return err
	}
	oldCurrentID := a.state.CurrentID
	oldNextSwitchAt := a.state.NextSwitchAt
	oldScheduleStartedAt := a.state.ScheduleStartedAt
	oldScheduleHours := a.state.ScheduleHours
	delete(a.state.Images, record.ID)
	a.state.CurrentID = ""
	a.state.NextSwitchAt = time.Time{}
	a.state.ScheduleStartedAt = time.Time{}
	a.state.ScheduleHours = 0
	if err := a.saveStateLocked(); err != nil {
		a.state.Images[record.ID] = record
		a.state.CurrentID = oldCurrentID
		a.state.NextSwitchAt = oldNextSwitchAt
		a.state.ScheduleStartedAt = oldScheduleStartedAt
		a.state.ScheduleHours = oldScheduleHours
		a.mu.Unlock()
		return err
	}
	remaining := len(a.state.Images)
	a.mu.Unlock()

	paths := []string{
		a.archivePath(&copyOfRecord),
		filepath.Join(a.root, "favorites", copyOfRecord.FileName),
		filepath.Join(a.root, "favorites", copyOfRecord.ID[:20]+".json"),
		filepath.Join(a.root, "generated", copyOfRecord.ID[:20]+".jpg"),
		a.metadataPath(&copyOfRecord),
	}
	var cleanupErr error
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if remaining == 0 {
		a.status(fmt.Sprintf("已删除“%s”；归档已空，请下载图片", displayName(&copyOfRecord)))
		return cleanupErr
	}
	if err := a.switchWallpaperLocked(); err != nil {
		return errors.Join(cleanupErr, err)
	}
	return cleanupErr
}

func validateStoredRecord(record *ImageRecord) error {
	if len(record.ID) < 20 || record.FileName != record.ID[:20]+".jpg" || filepath.Base(record.FileName) != record.FileName {
		return fmt.Errorf("图片记录包含不安全的文件名")
	}
	return nil
}

// reapplyCurrentWallpaper regenerates the current image after an app update or
// restart without changing its rating, show count, or scheduled switch time.
func (a *App) reapplyCurrentWallpaper() error {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	a.mu.Lock()
	record := a.state.Images[a.state.CurrentID]
	if record == nil {
		a.mu.Unlock()
		return nil
	}
	copyOfRecord := *record
	next := a.state.NextSwitchAt
	a.mu.Unlock()
	outputPath, err := a.renderWallpaper(copyOfRecord)
	if err != nil {
		return fmt.Errorf("重新生成当前壁纸: %w", err)
	}
	if err := setWallpaper(outputPath); err != nil {
		return err
	}
	a.status(currentStatus(&copyOfRecord, time.Until(next)))
	return nil
}

func (a *App) openCurrentMetadata() error {
	a.mu.Lock()
	record := a.state.Images[a.state.CurrentID]
	if record == nil {
		a.mu.Unlock()
		return fmt.Errorf("当前没有壁纸")
	}
	copyOfRecord := *record
	a.mu.Unlock()
	if err := validateStoredRecord(&copyOfRecord); err != nil {
		return err
	}
	return openFile(a.metadataPath(&copyOfRecord))
}

func chooseImage(images map[string]*ImageRecord, currentID string) (*ImageRecord, error) {
	return chooseImageFromSource(images, currentID, "")
}

func chooseImageFromSource(images map[string]*ImageRecord, currentID, source string) (*ImageRecord, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("归档中还没有图片，请先下载")
	}
	matchingCount := 0
	for _, record := range images {
		if source == "" || storageSource(record.Source) == source {
			matchingCount++
		}
	}
	candidates := make([]*ImageRecord, 0, len(images))
	for id, record := range images {
		if source != "" && storageSource(record.Source) != source {
			continue
		}
		if matchingCount > 1 && id == currentID {
			continue
		}
		candidates = append(candidates, record)
	}
	if len(candidates) == 0 {
		if source != "" {
			return nil, fmt.Errorf("归档中还没有可切换的 %s 图片，请先下载", source)
		}
		return nil, fmt.Errorf("归档中还没有可切换的图片，请先下载")
	}
	// Pick from the least-shown, least-recently-shown group. This gives every
	// archived image a turn before starting another cycle. Rating deliberately
	// does not affect selection: it controls how long an image stays visible.
	best := make([]*ImageRecord, 0, len(candidates))
	for _, record := range candidates {
		if len(best) == 0 || fairerCandidate(record, best[0]) {
			best = best[:0]
			best = append(best, record)
			continue
		}
		if sameRotationPriority(record, best[0]) {
			best = append(best, record)
		}
	}
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(best))))
	if err != nil {
		return nil, err
	}
	copyOfRecord := *best[int(n.Int64())]
	return &copyOfRecord, nil
}

func fairerCandidate(candidate, currentBest *ImageRecord) bool {
	if candidate.ShowCount != currentBest.ShowCount {
		return candidate.ShowCount < currentBest.ShowCount
	}
	return candidate.LastShownAt.Before(currentBest.LastShownAt)
}

func sameRotationPriority(left, right *ImageRecord) bool {
	return left.ShowCount == right.ShowCount && left.LastShownAt.Equal(right.LastShownAt)
}

func (a *App) rateCurrent(delta int) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	record := a.state.Images[a.state.CurrentID]
	if record == nil {
		return 0, fmt.Errorf("当前没有壁纸")
	}
	record.Rating = clamp(record.Rating+delta, 1, 5)
	a.setScheduleLocked(time.Now(), record.Rating)
	if err := writeJSON(a.metadataPath(record), record); err != nil {
		return 0, err
	}
	if err := a.saveStateLocked(); err != nil {
		return 0, err
	}
	a.status(fmt.Sprintf("已将“%s”设为 %d 星", displayName(record), record.Rating))
	return record.Rating, nil
}

func (a *App) displayDuration(rating int) time.Duration {
	if clamp(rating, 1, 5) == 3 {
		return time.Duration(a.config.DefaultSwitchHours) * time.Hour
	}
	hours := a.config.RatingDisplayHours[strconv.Itoa(clamp(rating, 1, 5))]
	return time.Duration(hours) * time.Hour
}

func (a *App) setScheduleLocked(startedAt time.Time, rating int) {
	duration := a.displayDuration(rating)
	a.state.ScheduleStartedAt = startedAt
	a.state.ScheduleHours = int(duration / time.Hour)
	a.state.NextSwitchAt = startedAt.Add(duration)
}

// reconcileSchedule migrates persisted schedules created with older defaults
// and also applies future config changes without resetting elapsed display time.
func (a *App) reconcileSchedule() error {
	record := a.state.Images[a.state.CurrentID]
	if record == nil {
		return nil
	}
	wantedHours := int(a.displayDuration(record.Rating) / time.Hour)
	startedAt := a.state.ScheduleStartedAt
	if startedAt.IsZero() {
		startedAt = record.LastShownAt
	}
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	wantedNext := startedAt.Add(time.Duration(wantedHours) * time.Hour)
	if a.state.ScheduleHours == wantedHours && a.state.ScheduleStartedAt.Equal(startedAt) && a.state.NextSwitchAt.Equal(wantedNext) {
		return nil
	}
	a.state.ScheduleStartedAt = startedAt
	a.state.ScheduleHours = wantedHours
	a.state.NextSwitchAt = wantedNext
	return a.saveStateLocked()
}

func (a *App) maintenance(ctx context.Context) {
	a.mu.Lock()
	count := len(a.state.Images)
	dueDownload := a.state.LastDownloadCheck.IsZero() || time.Since(a.state.LastDownloadCheck) >= time.Duration(a.config.AutoDownloadHours)*time.Hour
	dueSwitch := a.state.CurrentID == "" || a.state.NextSwitchAt.IsZero() || !time.Now().Before(a.state.NextSwitchAt)
	a.mu.Unlock()
	if count == 0 || dueDownload {
		if _, err := a.downloadSpotlightImages(ctx); err != nil {
			logError("自动下载 Spotlight 失败", err)
			a.status("自动下载 Spotlight 失败，稍后重试")
		}
		if strings.TrimSpace(a.config.UnsplashAccessKey) != "" {
			if _, err := a.downloadUnsplashImages(ctx); err != nil {
				logError("自动下载 Unsplash 失败", err)
				a.status("自动下载 Unsplash 失败，稍后重试")
			}
		}
	}
	a.mu.Lock()
	count = len(a.state.Images)
	a.mu.Unlock()
	if count > 0 && dueSwitch {
		if err := a.switchWallpaper(); err != nil {
			logError("自动切换失败", err)
			a.status("自动切换失败，稍后重试")
		}
	}
}

func (a *App) runScheduler(ctx context.Context) {
	if err := a.reapplyCurrentWallpaper(); err != nil {
		logError("恢复当前壁纸失败", err)
	}
	a.maintenance(ctx)
	ticker := time.NewTicker(time.Duration(a.config.CheckIntervalMinutes) * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			a.maintenance(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func displayName(record *ImageRecord) string {
	if record.Title != "" {
		return record.Title
	}
	return record.FileName
}

func currentStatus(record *ImageRecord, remaining time.Duration) string {
	return fmt.Sprintf("当前：%s（%d 星，%s后切换）", displayName(record), record.Rating, friendlyDuration(remaining))
}

func friendlyDuration(d time.Duration) string {
	if d < 0 {
		return "现在"
	}
	if d >= 48*time.Hour {
		return fmt.Sprintf("%d 天", int(d.Hours()/24+0.5))
	}
	if d >= time.Hour {
		return fmt.Sprintf("%d 小时", int(d.Hours()+0.5))
	}
	return fmt.Sprintf("%d 分钟", int(d.Minutes()+0.5))
}

func executableRoot() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}
