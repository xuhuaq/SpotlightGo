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
	"sort"
	"strings"
	"time"
)

type spotlightItem struct {
	URL         string
	Title       string
	Description string
	Copyright   string
	Source      string
	SourceID    string
	SourcePage  string
	Author      string
	AuthorPage  string
	Likes       int
	DownloadAPI string
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
			Source:      "spotlight",
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Spotlight API 没有返回可用的横向图片")
	}
	return result, nil
}

type unsplashPhoto struct {
	ID             string `json:"id"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Likes          int    `json:"likes"`
	Description    string `json:"description"`
	AltDescription string `json:"alt_description"`
	URLs           struct {
		Raw string `json:"raw"`
	} `json:"urls"`
	Links struct {
		HTML             string `json:"html"`
		DownloadLocation string `json:"download_location"`
	} `json:"links"`
	User struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	} `json:"user"`
}

func firstLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func (a *App) downloadSpotlightImages(ctx context.Context) (int, error) {
	a.workMu.Lock()
	defer a.workMu.Unlock()
	a.status("正在下载 Spotlight 图片…")
	client := &http.Client{Timeout: 45 * time.Second}
	items, err := a.fetchSpotlightItems(ctx, client)
	if err != nil {
		return 0, err
	}
	return a.archiveItems(ctx, client, items, "Spotlight")
}

func (a *App) downloadUnsplashImages(ctx context.Context) (int, error) {
	if strings.TrimSpace(a.config.UnsplashAccessKey) == "" {
		a.status("尚未配置 Unsplash Access Key；请填写 config.json 后重启程序")
		return 0, openFile(filepath.Join(a.root, "config.json"))
	}
	a.workMu.Lock()
	defer a.workMu.Unlock()
	a.status("正在下载 Unsplash 精选图片…")
	client := &http.Client{Timeout: 45 * time.Second}
	a.mu.Lock()
	page := a.state.UnsplashPage
	a.mu.Unlock()
	if page < 1 || page > 10 {
		page = 1
	}
	items, err := a.fetchUnsplashItems(ctx, client, page)
	if err != nil {
		return 0, err
	}
	a.mu.Lock()
	a.state.UnsplashPage = page%10 + 1
	a.mu.Unlock()
	return a.archiveItems(ctx, client, items, "Unsplash")
}

func (a *App) archiveItems(ctx context.Context, client *http.Client, items []spotlightItem, sourceName string) (int, error) {
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
	err := a.saveStateLocked()
	total := len(a.state.Images)
	a.mu.Unlock()
	if err != nil {
		return added, err
	}
	a.status(fmt.Sprintf("%s 下载完成：新增 %d 张，共 %d 张", sourceName, added, total))
	return added, nil
}

func (a *App) fetchSpotlightItems(ctx context.Context, client *http.Client) ([]spotlightItem, error) {

	params := url.Values{}
	params.Set("placement", "88000820")
	params.Set("bcnt", "4")
	params.Set("country", a.config.Country)
	params.Set("locale", a.config.Locale)
	params.Set("fmt", "json")
	endpoint := "https://fd.api.iris.microsoft.com/v4/api/selection?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) SpotlightGo/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Spotlight API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Spotlight API 返回 HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	return parseAPIResponse(body)
}

func (a *App) fetchUnsplashItems(ctx context.Context, client *http.Client, page int) ([]spotlightItem, error) {
	endpoint := fmt.Sprintf("https://api.unsplash.com/topics/%s/photos?page=%d&per_page=30&orientation=landscape&order_by=popular", url.PathEscape(a.config.UnsplashTopic), page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Client-ID "+strings.TrimSpace(a.config.UnsplashAccessKey))
	req.Header.Set("Accept-Version", "v1")
	req.Header.Set("User-Agent", "SpotlightGo/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 Unsplash API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, unsplashStatusError(resp)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var photos []unsplashPhoto
	if err := json.Unmarshal(body, &photos); err != nil {
		return nil, fmt.Errorf("解析 Unsplash 响应: %w", err)
	}
	sort.SliceStable(photos, func(i, j int) bool {
		i4K := photos[i].Width >= a.config.UnsplashMinWidth && photos[i].Height >= a.config.UnsplashMinHeight
		j4K := photos[j].Width >= a.config.UnsplashMinWidth && photos[j].Height >= a.config.UnsplashMinHeight
		if i4K != j4K {
			return i4K
		}
		if photos[i].Likes != photos[j].Likes {
			return photos[i].Likes > photos[j].Likes
		}
		return int64(photos[i].Width)*int64(photos[i].Height) > int64(photos[j].Width)*int64(photos[j].Height)
	})

	items := make([]spotlightItem, 0, a.config.UnsplashDownloads)
	for _, photo := range photos {
		if photo.ID == "" || photo.Width <= photo.Height || photo.URLs.Raw == "" || photo.Links.DownloadLocation == "" {
			continue
		}
		downloadURL, err := unsplashJPEGURL(photo.URLs.Raw)
		if err != nil {
			continue
		}
		title := strings.TrimSpace(photo.Description)
		if title == "" {
			title = strings.TrimSpace(photo.AltDescription)
		}
		sourcePage, _ := addReferralParameters(photo.Links.HTML)
		authorPage, _ := addReferralParameters("https://unsplash.com/@" + url.PathEscape(photo.User.Username))
		items = append(items, spotlightItem{
			URL:         downloadURL,
			Title:       title,
			Description: strings.TrimSpace(photo.Description),
			Copyright:   fmt.Sprintf("Photo by %s on Unsplash · %d likes", photo.User.Name, photo.Likes),
			Source:      "unsplash",
			SourceID:    photo.ID,
			SourcePage:  sourcePage,
			Author:      photo.User.Name,
			AuthorPage:  authorPage,
			Likes:       photo.Likes,
			DownloadAPI: photo.Links.DownloadLocation,
		})
		if len(items) >= a.config.UnsplashDownloads {
			break
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("Unsplash 没有返回可用的横向图片")
	}
	return items, nil
}

func addReferralParameters(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("无效的 Unsplash 来源地址")
	}
	query := parsed.Query()
	query.Set("utm_source", "spotlightgo")
	query.Set("utm_medium", "referral")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func unsplashStatusError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("Unsplash Access Key 无效或未获授权（HTTP %s）", resp.Status)
	case http.StatusForbidden, http.StatusTooManyRequests:
		return fmt.Errorf("Unsplash API 配额已用完或请求受限（HTTP %s）", resp.Status)
	default:
		return fmt.Errorf("Unsplash API 返回 HTTP %s", resp.Status)
	}
}

func unsplashJPEGURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return "", fmt.Errorf("无效的 Unsplash 图片地址")
	}
	query := parsed.Query()
	query.Set("fm", "jpg")
	query.Set("q", "95")
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (a *App) downloadOne(ctx context.Context, client *http.Client, item spotlightItem) (bool, error) {
	if item.SourceID != "" {
		a.mu.Lock()
		for _, existing := range a.state.Images {
			if existing.Source == item.Source && existing.SourceID == item.SourceID {
				existing.Likes = item.Likes
				if item.DownloadAPI != "" {
					existing.DownloadAPI = item.DownloadAPI
				}
				updated := *existing
				a.mu.Unlock()
				if err := writeJSON(filepath.Join(a.root, "metadata", existing.ID[:20]+".json"), updated); err != nil {
					return false, err
				}
				return false, nil
			}
		}
		a.mu.Unlock()
	}
	if item.DownloadAPI != "" {
		if err := a.trackUnsplashDownload(ctx, client, item.DownloadAPI); err != nil {
			logError("报告 Unsplash 下载失败，继续保存图片", err)
		}
	}
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
		existing.Likes = item.Likes
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
		Source:       item.Source,
		SourceID:     item.SourceID,
		SourcePage:   item.SourcePage,
		Author:       item.Author,
		AuthorPage:   item.AuthorPage,
		Likes:        item.Likes,
		DownloadAPI:  item.DownloadAPI,
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

func (a *App) trackUnsplashDownload(ctx context.Context, client *http.Client, endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "api.unsplash.com") {
		return fmt.Errorf("拒绝向非 Unsplash API 地址发送下载统计")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Client-ID "+strings.TrimSpace(a.config.UnsplashAccessKey))
	req.Header.Set("Accept-Version", "v1")
	req.Header.Set("User-Agent", "SpotlightGo/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("报告 Unsplash 下载: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return unsplashStatusError(resp)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return nil
}
