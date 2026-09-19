# SpotlightGo

一个轻量的 Windows 壁纸托盘程序：从 Spotlight 和可选的 Unsplash 精选源下载高分辨率图片、收藏原图，并在多显示器上安全显示标题。

## 功能

- 从 Windows Spotlight API 下载最高 4K 的横向图片。
- 可选接入 Unsplash Wallpapers 精选主题，优先下载达到 4K、点赞数高的热门横向图片。
- 原始图片永久保存在本地，不执行缓存淘汰或自动删除。
- 按 SHA-256 去重，并保存标题、完整介绍、版权、评分和展示历史。
- Spotlight 图片在保持原始像素尺寸的前提下，只在右上角绘制白色单行标题；Unsplash 图片不叠加任何文字。
- 根据所有显示器的宽高比计算共同可见区域，避免 Windows“填充”模式裁掉标题。
- 公平轮换本地图片，同一轮中尽量不重复；喜欢的图片展示更久。
- 托盘操作和当前用户级开机自启，无需管理员权限。
- EXE 与托盘使用同一枚原创 CC0 图标。

## 系统要求

- Windows 10 或 Windows 11，x64。
- 可访问 Microsoft Spotlight API 与图片 CDN。
- 程序为便携版，请解压到具有写权限的普通目录，不建议放入 `Program Files`。

## 安装和使用

从 GitHub 的 [Releases](../../releases/latest) 下载 `SpotlightGo-*-windows-amd64.zip`，解压后双击 `SpotlightGo.exe`。程序不会打开主窗口，图标会出现在 Windows 通知区域。

托盘菜单：

- **下载Spotlight图片**：单独请求 Windows Spotlight 并永久归档。
- **下载Unsplash图片**：单独下载最多 8 张精选图片；未配置 Access Key 时直接打开 `config.json`，填写后重启程序生效。
- **切换Spotlight图片**：只在 Spotlight 归档中选择展示次数最少、最久未展示的图片。
- **切换Unsplash图片**：只在 Unsplash 归档中选择展示次数最少、最久未展示的图片。
- **喜欢，收藏**：设为 5 星、延长展示时间，并将原图与同名 JSON 信息复制到 `favorites\`。
- **不喜欢，删除，下一张**：删除当前原图、生成图、收藏副本和元数据，然后立即换图。
- **打开程序目录**：查看归档、配置、状态和日志。
- **开机自启**：添加或移除当前用户的 Windows 登录启动项。
- **退出**：退出托盘程序。

鼠标悬停托盘图标会显示当前图片的标题、评分和下次切换时间；左键单击图标会用记事本打开当前图片的 JSON 信息，右键仍打开托盘菜单。

程序重启时会重新生成并应用当前壁纸，因此版本升级后的叠字样式会立即生效。

## 评分和停留时间

新图片默认为 3 星：

| 评分 | 单次展示时间 |
|---:|---:|
| 1 星 | 2 小时 |
| 2 星 | 8 小时 |
| 3 星 | 6 小时 |
| 4 星 | 3 天 |
| 5 星 | 7 天 |

评分只改变单次停留时间，不改变轮换顺序。程序优先选择展示次数最少的图片；次数相同时选择最久未展示的图片，使归档中的图片依次获得展示机会。

## 数据文件

数据位于 `SpotlightGo.exe` 所在目录：

| 路径 | 用途 |
|---|---|
| `archive\spotlight\` | Spotlight 原图及同名 JSON 元数据，永久保留 |
| `archive\unsplash\` | Unsplash 原图及同名 JSON 元数据，永久保留 |
| `favorites\` | 通过“喜欢，收藏”保存的原图副本 |
| `generated\` | 保持原尺寸的叠字壁纸 |
| `config.json` | 语言、下载间隔、JPEG 品质和停留时间 |
| `state.json` | 当前壁纸、展示记录和下次切换时间 |
| `spotlight.log` | 运行日志 |

默认请求简体中文内容（`zh-CN`/`CN`）。首次运行会生成 `config.json`；也可以参考 [config.example.json](config.example.json)。修改配置后重启程序生效。

## Unsplash 图片源

Unsplash 默认关闭，因为其官方 API 要求每位使用者提供 Access Key。前往 [Unsplash Developers](https://unsplash.com/developers) 注册应用，将 Access Key 写入 `config.json`：

```json
"unsplash_access_key": "你的 Access Key"
```

重启后，“下载Unsplash图片”会读取 `wallpapers` 精选主题的热门横向图片，每次最多保存 8 张；候选图优先按是否达到 3840×2160、点赞数和总像素数排序。可通过 `unsplash_topic`、`unsplash_downloads`、`unsplash_min_width` 和 `unsplash_min_height` 调整。程序直接使用 API 返回的图片地址，并在保存及应用壁纸时调用其下载统计接口；图片作者、来源页面和点赞数会写入元数据，但不再追加到托盘状态文字。Unsplash 壁纸不叠加标题或署名文字。

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

便携工具链位于 `.tools\`，模块与编译缓存分别位于 `.gomodcache\` 和 `.gocache\`；这些目录会保留，方便后续重新编译，但已被 `.gitignore` 排除。默认切换时间由 `config.json` 的 `default_switch_hours` 控制，默认是 6 小时。

发布包会自动安装并保留 UPX，然后在 Go 的 `-s -w` 基础上进一步压缩 EXE。普通构建在 UPX 已安装时也会压缩；如需排查杀毒软件误报，可使用：

```powershell
.\build.ps1 -NoCompress
```

图标原稿、多尺寸 ICO 和生成脚本位于 `assets\` 与 `generate-icon.ps1`。图标按 [CC0 1.0](assets/ICON_LICENSE.md) 提供。

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

SpotlightGo 不包含自身遥测、账户系统或广告。默认仅连接 Microsoft Spotlight API 和图片 CDN；配置 Unsplash 后还会连接其 API、图片 CDN，并按官方要求报告图片下载事件。图片的介绍和版权属于各自权利人；下载与使用时请遵守相应服务条款及所在地区适用的规定。

## 致谢

Spotlight API 的端点分析、字段说明和原程序行为参考了 [ORelio/Spotlight-Downloader](https://github.com/ORelio/Spotlight-Downloader)。SpotlightGo 是独立的 Go 实现，与 Microsoft 没有关联或背书关系。

Unsplash 的多来源组织、用户自备 Access Key、原始质量图片和下载统计处理参考了 MIT 许可的 [Halfaxas/Splashy](https://github.com/Halfaxas/Splashy)；SpotlightGo 保留轻量托盘形态，并增加本地公平轮换、4K/点赞排序和永久归档。

第三方依赖及许可证见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。

## 许可证

本项目采用 [CDDL-1.0](LICENSE)。发布可执行文件时，源代码可从对应 GitHub Release 标签获取。
