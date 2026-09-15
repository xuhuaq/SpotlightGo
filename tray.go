// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"fyne.io/systray"
)

func runTray(app *App) {
	ctx, cancel := context.WithCancel(context.Background())
	systray.Run(func() {
		systray.SetIcon(trayIcon())
		systray.SetTitle("SpotlightGo")
		systray.SetTooltip("SpotlightGo " + version)

		statusItem := systray.AddMenuItem("正在启动…", "当前状态")
		statusItem.Disable()
		systray.AddSeparator()
		switchItem := systray.AddMenuItem("切换图片", "立即切换到另一张本地图片")
		systray.AddSeparator()
		downloadSpotlightItem := systray.AddMenuItem("下载Spotlight图片", "从 Windows Spotlight 下载并永久归档")
		downloadUnsplashItem := systray.AddMenuItem("下载Unsplash图片", "下载精选高分辨率高赞图片；未配置密钥时打开配置文件")
		systray.AddSeparator()
		likeItem := systray.AddMenuItem("喜欢，收藏", "设为 5 星并将原图保存到 favorites 目录")
		dislikeItem := systray.AddMenuItem("不喜欢，删除", "删除当前图片及元数据并立即换一张")
		systray.AddSeparator()
		openItem := systray.AddMenuItem("打开程序目录", "打开归档、配置和状态文件所在目录")
		startupItem := systray.AddMenuItemCheckbox("开机自启", "登录 Windows 后自动运行", startupEnabled())
		systray.AddSeparator()
		quitItem := systray.AddMenuItem("退出", "退出 SpotlightGo")

		app.setStatus = func(message string) {
			message = strings.ReplaceAll(message, "\n", " ")
			if len([]rune(message)) > 70 {
				message = string([]rune(message)[:70]) + "…"
			}
			statusItem.SetTitle(message)
			systray.SetTooltip("SpotlightGo " + version + " · " + message)
			log.Print(message)
		}

		go app.runScheduler(ctx)
		go func() {
			for {
				select {
				case <-downloadSpotlightItem.ClickedCh:
					go func() {
						if _, err := app.downloadSpotlightImages(ctx); err != nil {
							reportTrayError(app, "Spotlight 下载失败", err)
						}
					}()
				case <-downloadUnsplashItem.ClickedCh:
					go func() {
						if _, err := app.downloadUnsplashImages(ctx); err != nil {
							reportTrayError(app, "Unsplash 下载失败", err)
						}
					}()
				case <-switchItem.ClickedCh:
					go func() {
						if err := app.switchWallpaper(); err != nil {
							reportTrayError(app, "切换失败", err)
						}
					}()
				case <-likeItem.ClickedCh:
					if err := app.favoriteCurrent(); err != nil {
						reportTrayError(app, "收藏失败", err)
					}
				case <-dislikeItem.ClickedCh:
					go func() {
						if err := app.deleteCurrentAndSwitch(); err != nil {
							reportTrayError(app, "删除或切换失败", err)
						}
					}()
				case <-openItem.ClickedCh:
					if err := openDirectory(app.root); err != nil {
						reportTrayError(app, "打开目录失败", err)
					}
				case <-startupItem.ClickedCh:
					enable := !startupItem.Checked()
					if err := setStartup(enable); err != nil {
						reportTrayError(app, "修改开机自启失败", err)
						continue
					}
					if enable {
						startupItem.Check()
						app.status("已启用开机自启")
					} else {
						startupItem.Uncheck()
						app.status("已关闭开机自启")
					}
				case <-quitItem.ClickedCh:
					cancel()
					systray.Quit()
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	}, func() {
		cancel()
	})
}

func reportTrayError(app *App, action string, err error) {
	logError(action, err)
	app.status(fmt.Sprintf("%s：%v", action, err))
}
