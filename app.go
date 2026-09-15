// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func (a *App) switchWallpaper() error {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	a.status("正在选择壁纸…")

	a.mu.Lock()
	record, err := chooseImage(a.state.Images, a.state.CurrentID)
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

	now := time.Now()
	a.mu.Lock()
	current := a.state.Images[record.ID]
	current.LastShownAt = now
	current.ShowCount++
	a.state.CurrentID = record.ID
	a.state.NextSwitchAt = now.Add(a.displayDuration(current.Rating))
	err = a.saveStateLocked()
	next := a.state.NextSwitchAt
	a.mu.Unlock()
	if err != nil {
		return err
	}
	a.status(fmt.Sprintf("当前：%s（%d 星，%s 后切换）", displayName(current), current.Rating, friendlyDuration(time.Until(next))))
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
	a.status(fmt.Sprintf("当前：%s（%d 星，%s 后切换）", displayName(&copyOfRecord), copyOfRecord.Rating, friendlyDuration(time.Until(next))))
	return nil
}

func chooseImage(images map[string]*ImageRecord, currentID string) (*ImageRecord, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("归档中还没有图片，请先下载")
	}
	candidates := make([]*ImageRecord, 0, len(images))
	for id, record := range images {
		if len(images) > 1 && id == currentID {
			continue
		}
		candidates = append(candidates, record)
	}
	weights := []int{0, 1, 2, 4, 10, 20}
	total := 0
	for _, record := range candidates {
		rating := clamp(record.Rating, 1, 5)
		total += weights[rating]
	}
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(total)))
	if err != nil {
		return nil, err
	}
	pick := int(n.Int64())
	for _, record := range candidates {
		pick -= weights[clamp(record.Rating, 1, 5)]
		if pick < 0 {
			copyOfRecord := *record
			return &copyOfRecord, nil
		}
	}
	return nil, fmt.Errorf("无法选择图片")
}

func (a *App) rateCurrent(delta int) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	record := a.state.Images[a.state.CurrentID]
	if record == nil {
		return 0, fmt.Errorf("当前没有壁纸")
	}
	record.Rating = clamp(record.Rating+delta, 1, 5)
	a.state.NextSwitchAt = time.Now().Add(a.displayDuration(record.Rating))
	if err := writeJSON(filepath.Join(a.root, "metadata", record.ID[:20]+".json"), record); err != nil {
		return 0, err
	}
	if err := a.saveStateLocked(); err != nil {
		return 0, err
	}
	a.status(fmt.Sprintf("已将“%s”设为 %d 星", displayName(record), record.Rating))
	return record.Rating, nil
}

func (a *App) displayDuration(rating int) time.Duration {
	hours := a.config.RatingDisplayHours[strconv.Itoa(clamp(rating, 1, 5))]
	return time.Duration(hours) * time.Hour
}

func (a *App) maintenance(ctx context.Context) {
	a.mu.Lock()
	count := len(a.state.Images)
	dueDownload := a.state.LastDownloadCheck.IsZero() || time.Since(a.state.LastDownloadCheck) >= time.Duration(a.config.AutoDownloadHours)*time.Hour
	dueSwitch := a.state.CurrentID == "" || a.state.NextSwitchAt.IsZero() || !time.Now().Before(a.state.NextSwitchAt)
	a.mu.Unlock()
	if count == 0 || dueDownload {
		if _, err := a.downloadImages(ctx); err != nil {
			logError("自动下载失败", err)
			a.status("自动下载失败，稍后重试")
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
