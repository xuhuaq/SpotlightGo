# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

## [0.1.1] - 2026-09-14

### Changed

- Use fair least-shown/least-recently-shown wallpaper rotation to avoid repeats within a cycle.
- Let ratings control display duration only, instead of also increasing selection frequency.
- Add local favorite copies and a delete-current-then-switch tray action.
- Add an optional Unsplash popular-wallpaper source with 4K/likes prioritization and compliant download tracking.
- Follow Splashy's resilient Unsplash pattern: user-provided keys, non-blocking tracking failures, author links, and usage tracking when a wallpaper is applied.
- Split Spotlight and Unsplash into separate tray download commands; open `config.json` when the Unsplash key is missing.
- Never overlay titles on Unsplash wallpapers, remove the current-source command, and rename the favorite command.
- Use one original CC0 multi-size icon for both the Windows executable and tray.
- Add a configurable six-hour default switch interval and retained UPX release compression.
- Reconcile persisted legacy schedules at startup so the six-hour default takes effect immediately without resetting elapsed display time.

## [0.1.0] - 2026-09-14

### Added

- Windows notification-area application with download, switch, rating, folder and startup commands.
- Permanent SHA-256-deduplicated Spotlight image archive and JSON metadata.
- Rating-weighted selection and rating-dependent display duration.
- Original-resolution title rendering without modifying archived images.
- Multi-monitor safe-title placement for Windows Fill mode.
- Portable Go build scripts and Windows GitHub Actions workflows.
