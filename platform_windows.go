//go:build windows

// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const startupValueName = "SpotlightGo"

var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procEnumDisplayMonitors   = user32.NewProc("EnumDisplayMonitors")
)

type winRect struct {
	Left, Top, Right, Bottom int32
}

func monitorDimensions() []image.Point {
	var result []image.Point
	callback := syscall.NewCallback(func(_ uintptr, _ uintptr, rect *winRect, _ uintptr) uintptr {
		if rect != nil {
			width := int(rect.Right - rect.Left)
			height := int(rect.Bottom - rect.Top)
			if width > 0 && height > 0 {
				result = append(result, image.Pt(width, height))
			}
		}
		return 1
	})
	resultCode, _, _ := procEnumDisplayMonitors.Call(0, 0, callback, 0)
	if resultCode == 0 || len(result) == 0 {
		return nil
	}
	return result
}

func setWallpaper(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Control Panel\Desktop`, registry.SET_VALUE)
	if err == nil {
		_ = key.SetStringValue("WallpaperStyle", "10")
		_ = key.SetStringValue("TileWallpaper", "0")
		key.Close()
	}
	p, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return err
	}
	result, _, callErr := procSystemParametersInfoW.Call(20, 0, uintptr(unsafe.Pointer(p)), 0x01|0x02)
	if result == 0 {
		return fmt.Errorf("Windows 设置壁纸失败: %w", callErr)
	}
	return nil
}

func openDirectory(path string) error {
	return exec.Command("explorer.exe", path).Start()
}

func startupEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(startupValueName)
	return err == nil && value != ""
}

func setStartup(enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if !enabled {
		err := key.DeleteValue(startupValueName)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	command := fmt.Sprintf("\"%s\" --background", exe)
	return key.SetStringValue(startupValueName, command)
}

func ensureSingleInstance() (func(), bool) {
	name, _ := windows.UTF16PtrFromString(`Local\SpotlightGo.Tray.Instance`)
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			windows.CloseHandle(handle)
		}
		return func() {}, false
	}
	if err != nil {
		logError("创建单实例锁失败", err)
		return func() {}, true
	}
	return func() { windows.CloseHandle(handle) }, true
}

func hideConsoleIfPresent() {
	// Release builds use -H=windowsgui. Kept as a hook for debug builds.
}
