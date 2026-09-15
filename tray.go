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
		downloadItem := systray.AddMenuItem("下载图片", "从 Windows Spotlight 下载并永久归档")
		switchItem := systray.AddMenuItem("切换图片", "立即切换到另一张本地图片")
		systray.AddSeparator()
		likeItem := systray.AddMenuItem("喜欢当前图片 👍", "提高评分并延长展示时间")
		dislikeItem := systray.AddMenuItem("不喜欢并切换 👎", "降低评分并立即换一张")
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
				case <-downloadItem.ClickedCh:
					go func() {
						if _, err := app.downloadImages(ctx); err != nil {
							reportTrayError(app, "下载失败", err)
						}
					}()
				case <-switchItem.ClickedCh:
					go func() {
						if err := app.switchWallpaper(); err != nil {
							reportTrayError(app, "切换失败", err)
						}
					}()
				case <-likeItem.ClickedCh:
					if _, err := app.rateCurrent(1); err != nil {
						reportTrayError(app, "评分失败", err)
					}
				case <-dislikeItem.ClickedCh:
					go func() {
						if _, err := app.rateCurrent(-1); err != nil {
							reportTrayError(app, "评分失败", err)
							return
						}
						if err := app.switchWallpaper(); err != nil {
							reportTrayError(app, "切换失败", err)
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
