// Copyright 2026 SpotlightGo contributors.
// SPDX-License-Identifier: CDDL-1.0

package main

import (
	_ "embed"
)

//go:embed assets/spotlightgo.ico
var applicationIcon []byte

func trayIcon() []byte {
	return applicationIcon
}
