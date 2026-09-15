# Contributing

Bug reports and focused pull requests are welcome. Please do not include downloaded images, personal metadata, state files, logs, compiler caches, or compiled executables in commits.

Before submitting a pull request on Windows:

```powershell
go fmt ./...
go test ./...
go vet ./...
```

Keep Windows-specific calls in `platform_windows.go`, preserve archived originals, and add a regression test for changes to image sizing, title placement, state migration, or selection behavior.
