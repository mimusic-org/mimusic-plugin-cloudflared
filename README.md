# Cloudflared 隧道插件

通过 Cloudflare Tunnel 将本地 MiMusic 服务暴露到公网，无需公网 IP 和端口转发即可从外部访问。

## 功能特性

- **隧道管理**：启动/停止 Cloudflare Tunnel，将本地服务暴露到公网
- **自动下载**：自动从 GitHub 下载对应平台的 cloudflared 二进制文件
- **手动上传**：支持手动上传 cloudflared 二进制文件
- **实时日志**：查看隧道运行日志和状态
- **URL 提取**：自动提取并显示隧道公网访问地址
- **版本管理**：检查最新版本，支持版本更新

## 支持的架构

| 平台 | 架构 | 下载文件名 |
|------|------|-----------|
| macOS | AMD64 | `cloudflared-darwin-amd64.tgz` |
| macOS | ARM64 | `cloudflared-darwin-arm64.tgz` |
| Linux | AMD64 | `cloudflared-linux-amd64` |
| Linux | ARM64 | `cloudflared-linux-arm64` |
| Linux | ARMv7 | `cloudflared-linux-arm` |
| Windows | AMD64 | `cloudflared-windows-amd64.exe` |
| Windows | ARM64 | `cloudflared-windows-amd64.exe` |

## API 接口

### 状态管理

#### 获取状态
```http
GET /cloudflared/api/status
```

响应示例：
```json
{
  "installed": true,
  "running": true,
  "version": "cloudflared version 2024.12.2 (built 2024-12-17T10:38:05Z)\n"
}
```

### 隧道控制

#### 启动隧道
```http
POST /cloudflared/api/start
Content-Type: application/json

{
  "port": "58091"
}
```

响应示例：
```json
{
  "message": "cloudflared 已启动",
  "process_id": "cloudflared-tunnel"
}
```

#### 停止隧道
```http
POST /cloudflared/api/stop
```

响应示例：
```json
{
  "message": "cloudflared 已停止"
}
```

#### 获取运行输出
```http
GET /cloudflared/api/output
```

响应示例：
```json
{
  "stdout": "",
  "stderr": "2024-12-17T10:38:05Z INF ...",
  "running": true,
  "exit_code": 0
}
```

#### 获取隧道 URL
```http
GET /cloudflared/api/tunnel-url
```

响应示例：
```json
{
  "url": "https://xxxxx.trycloudflare.com"
}
```

### 下载管理

#### 获取最新 Release 信息
```http
GET /cloudflared/api/releases
```

#### 启动下载
```http
POST /cloudflared/api/download
Content-Type: application/json

{
  "platform": "linux-amd64"
}
```

响应示例：
```json
{
  "success": true,
  "message": "下载任务已启动",
  "task_id": "download-cloudflared",
  "version": "2024.12.2",
  "platform": "linux-amd64"
}
```

#### 获取下载状态
```http
GET /cloudflared/api/download/status
```

响应示例：
```json
{
  "status": "downloading",
  "downloaded_bytes": 10485760,
  "total_bytes": 20971520,
  "progress_percent": 50,
  "error": ""
}
```

### 手动上传

#### 上传二进制文件
```http
POST /cloudflared/api/upload
Content-Type: multipart/form-data

file: <cloudflared二进制文件>
```

响应示例：
```json
{
  "message": "上传成功",
  "filename": "cloudflared",
  "size": 10485760
}
```

## 使用流程

1. **安装插件**：在 MiMusic 插件管理页面上传 `cloudflared.wasm`
2. **下载 cloudflared**：进入插件设置页，点击"下载最新版本"自动下载，或手动上传二进制文件
3. **启动隧道**：在首页输入本地服务端口（默认 58091），点击"启动隧道"
4. **获取访问地址**：等待隧道启动完成，复制显示的公网 URL 即可从外部访问

## 构建

### 环境要求

- Go 1.26+
- 支持 WASI 的 Go 编译器

### 构建命令

```bash
# 显示帮助信息
make help

# 编译插件为 WASM 格式
make build

# 显示插件信息
make info

# 完整构建流程
make all
```

### 手动构建

```bash
GOOS=wasip1 GOARCH=wasm go build -o cloudflared.wasm -buildmode=c-shared
```

## 目录结构

```
.
├── main.go           # 插件入口，注册路由和生命周期管理
├── handlers.go       # HTTP 请求处理器
├── downloader.go     # cloudflared 下载逻辑
├── static/           # 静态资源文件
│   ├── index.html    # 管理页面
│   ├── css/          # 样式文件
│   ├── js/           # JavaScript 文件
│   └── fonts/        # 字体文件
├── Makefile          # 构建脚本
├── go.mod            # Go 模块定义
└── README.md         # 本文件
```

## 开发说明

### WASM 开发约束

- 使用 `//go:build wasip1` 构建标签
- 网络请求必须使用 `pluginhttp` 包替代标准库 `net/http`
- 文件操作通过 Host Function 代理实现
- 命令执行通过 `ExecuteCommand` Host Function 实现

### 依赖说明

```go
import (
    "github.com/mimusic-org/plugin/api/plugin"       // 插件框架
    "github.com/mimusic-org/plugin/api/pbplugin"     // Host Functions
    pluginhttp "github.com/mimusic-org/plugin/pkg/go-plugin-http/http"  // HTTP 客户端
)
```

## 注意事项

- 隧道 URL 是临时的，每次启动都会变化
- 免费版 Cloudflare Tunnel 有速率限制
- 首次下载需要从 GitHub 获取，可能需要等待一段时间
- Windows 平台上传 `.exe` 文件时会自动识别并保存为 `cloudflared.exe`
- macOS 平台下载的是 `.tgz` 压缩包，会自动解压提取二进制文件

## 许可证

[Apache License 2.0](LICENSE)
