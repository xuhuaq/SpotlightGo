// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
)

var version = "dev"

func main() {
	runtime.LockOSThread()
	root, err := executableRoot()
	if err != nil {
		return
	}
	logFile, err := os.OpenFile(filepath.Join(root, "spotlight.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Printf("SpotlightGo %s starting", version)
	releaseInstance, first := ensureSingleInstance()
	if !first {
		return
	}
	defer releaseInstance()

	app, err := newApp(root)
	if err != nil {
		logError("初始化失败", err)
		return
	}
	hideConsoleIfPresent()
	runTray(app)
}

func logError(message string, err error) {
	log.Printf("%s: %v", message, err)
}
