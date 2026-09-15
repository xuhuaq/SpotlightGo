# SpotlightGo

一个轻量的 Windows Spotlight 壁纸托盘程序：永久归档原图、按喜好控制展示时间，并在多显示器上安全显示标题。

## 功能

- 从 Windows Spotlight API 下载最高 4K 的横向图片。
- 原始图片永久保存在本地，不执行缓存淘汰或自动删除。
- 按 SHA-256 去重，并保存标题、完整介绍、版权、评分和展示历史。
- 在保持原始像素尺寸的前提下，只在右上角绘制白色单行标题。
- 根据所有显示器的宽高比计算共同可见区域，避免 Windows“填充”模式裁掉标题。
- 喜欢的图片展示更久、再次出现的概率更高。
- 托盘操作和当前用户级开机自启，无需管理员权限。

## 系统要求

- Windows 10 或 Windows 11，x64。
- 可访问 Microsoft Spotlight API 与图片 CDN。
- 程序为便携版，请解压到具有写权限的普通目录，不建议放入 `Program Files`。

## 安装和使用

从 GitHub 的 [Releases](../../releases/latest) 下载 `SpotlightGo-*-windows-amd64.zip`，解压后双击 `SpotlightGo.exe`。程序不会打开主窗口，图标会出现在 Windows 通知区域。

托盘菜单：

- **下载图片**：请求 Spotlight API，一次最多归档 4 张图片。
- **切换图片**：从本地归档中按评分权重选择另一张。
- **喜欢当前图片**：提高一星并延长当前展示时间。
- **不喜欢并切换**：降低一星并立即换图。
- **打开程序目录**：查看归档、配置、状态和日志。
- **开机自启**：添加或移除当前用户的 Windows 登录启动项。
- **退出**：退出托盘程序。

程序重启时会重新生成并应用当前壁纸，因此版本升级后的叠字样式会立即生效。

## 评分和停留时间

新图片默认为 3 星：

| 评分 | 单次展示时间 | 再次抽中权重 |
|---:|---:|---:|
| 1 星 | 2 小时 | 1 |
| 2 星 | 8 小时 | 2 |
| 3 星 | 24 小时 | 4 |
| 4 星 | 3 天 | 10 |
| 5 星 | 7 天 | 20 |

## 数据文件

数据位于 `SpotlightGo.exe` 所在目录：

| 路径 | 用途 |
|---|---|
| `archive\` | 下载的原始 JPG，永久保留 |
| `metadata\` | 每张图片的标题、介绍、版权和评分 |
| `generated\` | 保持原尺寸的叠字壁纸 |
| `config.json` | 语言、下载间隔、JPEG 品质和停留时间 |
| `state.json` | 当前壁纸、展示记录和下次切换时间 |
| `spotlight.log` | 运行日志 |

默认请求简体中文内容（`zh-CN`/`CN`）。首次运行会生成 `config.json`；也可以参考 [config.example.json](config.example.json)。修改配置后重启程序生效。

## 从源码编译

不需要 Visual Studio 或 C 编译器。需要 Go 1.23 或更高版本：

```powershell
.\build.ps1
```

如果电脑没有 Go，可以安装到项目内部：

```powershell
.\install-portable-go.ps1
.\build.ps1
```

便携工具链位于 `.tools\`，模块与编译缓存分别位于 `.gomodcache\` 和 `.gocache\`；这些目录会保留，方便后续重新编译，但已被 `.gitignore` 排除。

测试和静态检查：

```powershell
go test ./...
go vet ./...
```

推送到 GitHub 后，每次提交和拉取请求都会在 Windows runner 上测试并生成构建产物。推送形如 `v0.1.0` 的标签会自动创建带校验文件的 GitHub Release。

在本地生成相同结构的发布 ZIP：

```powershell
.\package-release.ps1 -Version v0.1.0
```

## 隐私和网络

SpotlightGo 不包含遥测、账户系统或广告。程序仅连接 Microsoft Spotlight API 和响应中提供的 Microsoft 图片地址。图片的介绍和版权属于各自权利人；下载与使用时请遵守所在地区适用的条款。

## 致谢

Spotlight API 的端点分析、字段说明和原程序行为参考了 [ORelio/Spotlight-Downloader](https://github.com/ORelio/Spotlight-Downloader)。SpotlightGo 是独立的 Go 实现，与 Microsoft 没有关联或背书关系。

第三方依赖及许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 许可证

本项目采用 [CDDL-1.0](LICENSE)。发布可执行文件时，源代码可从对应 GitHub Release 标签获取。
