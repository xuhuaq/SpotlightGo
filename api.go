// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type spotlightItem struct {
	URL         string
	Title       string
	Description string
	Copyright   string
}

type apiResponse struct {
	BatchResponse struct {
		Items []struct {
			Item string `json:"item"`
		} `json:"items"`
	} `json:"batchrsp"`
}

type nestedItem struct {
	Ad struct {
		LandscapeImage struct {
			Asset string `json:"asset"`
		} `json:"landscapeImage"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		Copyright     string `json:"copyright"`
		IconHoverText string `json:"iconHoverText"`
	} `json:"ad"`
}

func parseAPIResponse(data []byte) ([]spotlightItem, error) {
	var envelope apiResponse
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("解析 Spotlight 响应: %w", err)
	}
	var result []spotlightItem
	for _, wrapper := range envelope.BatchResponse.Items {
		var nested nestedItem
		if err := json.Unmarshal([]byte(wrapper.Item), &nested); err != nil {
			continue
		}
		ad := nested.Ad
		if !strings.HasPrefix(strings.ToLower(ad.LandscapeImage.Asset), "https://") {
			continue
		}
		title := strings.TrimSpace(ad.Title)
		if title == "" {
			title = firstLine(ad.IconHoverText)
		}
		result = append(result, spotlightItem{
			URL:         ad.LandscapeImage.Asset,
			Title:       title,
			Description: strings.TrimSpace(ad.Description),
			Copyright:   strings.TrimSpace(ad.Copyright),
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Spotlight API 没有返回可用的横向图片")
	}
	return result, nil
}

func firstLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func (a *App) downloadImages(ctx context.Context) (int, error) {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	a.status("正在连接 Spotlight…")

	params := url.Values{}
	params.Set("placement", "88000820")
	params.Set("bcnt", "4")
	params.Set("country", a.config.Country)
	params.Set("locale", a.config.Locale)
	params.Set("fmt", "json")
	endpoint := "https://fd.api.iris.microsoft.com/v4/api/selection?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SpotlightGo/1.0")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("请求 Spotlight API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Spotlight API 返回 HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return 0, err
	}
	items, err := parseAPIResponse(body)
	if err != nil {
		return 0, err
	}

	added := 0
	succeeded := 0
	for _, item := range items {
		wasAdded, err := a.downloadOne(ctx, client, item)
		if err != nil {
			logError("下载图片失败", err)
			continue
		}
		succeeded++
		if wasAdded {
			added++
		}
	}
	if succeeded == 0 {
		return 0, fmt.Errorf("本次返回的 %d 张图片均下载失败", len(items))
	}

	a.mu.Lock()
	a.state.LastDownloadCheck = time.Now()
	err = a.saveStateLocked()
	total := len(a.state.Images)
	a.mu.Unlock()
	if err != nil {
		return added, err
	}
	a.status(fmt.Sprintf("下载完成：新增 %d 张，共 %d 张", added, total))
	return added, nil
}

func (a *App) downloadOne(ctx context.Context, client *http.Client, item spotlightItem) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.URL, nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("HTTP %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (64<<20)+1))
	if err != nil {
		return false, err
	}
	if len(data) > 64<<20 {
		return false, fmt.Errorf("图片超过 64 MB 限制")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width < 1 || cfg.Height < 1 {
		return false, fmt.Errorf("下载内容不是有效图片")
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	fileName := id[:20] + ".jpg"

	a.mu.Lock()
	if existing, ok := a.state.Images[id]; ok {
		if item.Title != "" {
			existing.Title = item.Title
		}
		if item.Description != "" {
			existing.Description = item.Description
		}
		if item.Copyright != "" {
			existing.Copyright = item.Copyright
		}
		updated := *existing
		a.mu.Unlock()
		if err := writeJSON(filepath.Join(a.root, "metadata", id[:20]+".json"), updated); err != nil {
			return false, err
		}
		return false, nil
	}
	a.mu.Unlock()

	archivePath := filepath.Join(a.root, "archive", fileName)
	if err := os.WriteFile(archivePath, data, 0o644); err != nil {
		return false, err
	}
	record := &ImageRecord{
		ID:           id,
		FileName:     fileName,
		SourceURL:    item.URL,
		Title:        item.Title,
		Description:  item.Description,
		Copyright:    item.Copyright,
		Width:        cfg.Width,
		Height:       cfg.Height,
		Rating:       3,
		DownloadedAt: time.Now(),
	}
	if err := writeJSON(filepath.Join(a.root, "metadata", id[:20]+".json"), record); err != nil {
		return false, err
	}
	a.mu.Lock()
	a.state.Images[id] = record
	a.mu.Unlock()
	return true, nil
}
