//go:build wasip1
// +build wasip1

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mimusic-org/plugin/api/pbplugin"
	pluginhttp "github.com/mimusic-org/plugin/pkg/go-plugin-http/http"
)

// downloadTaskID 下载任务的固定标识符
const downloadTaskID = "download-cloudflared"

// platformMapping 平台到 cloudflared 下载文件名的映射
var platformMapping = map[string]struct {
	FileName     string
	NeedsExtract bool
}{
	"darwin-amd64":  {FileName: "cloudflared-darwin-amd64.tgz", NeedsExtract: true},
	"darwin-arm64":  {FileName: "cloudflared-darwin-arm64.tgz", NeedsExtract: true},
	"linux-amd64":   {FileName: "cloudflared-linux-amd64", NeedsExtract: false},
	"linux-arm64":   {FileName: "cloudflared-linux-arm64", NeedsExtract: false},
	"linux-armv7":   {FileName: "cloudflared-linux-arm", NeedsExtract: false},
	"windows-amd64": {FileName: "cloudflared-windows-amd64.exe", NeedsExtract: false},
	"windows-arm64": {FileName: "cloudflared-windows-amd64.exe", NeedsExtract: false}, // x86 模拟层兼容
}

// githubRelease GitHub release 信息
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Name    string        `json:"name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset GitHub release asset
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// downloadStartResult 异步下载启动结果
type downloadStartResult struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	TaskID   string `json:"task_id"`
	Version  string `json:"version,omitempty"`
	Platform string `json:"platform,omitempty"`
}

// getLatestRelease 获取 GitHub 最新 release 信息
func getLatestRelease() (*githubRelease, error) {
	resp, err := pluginhttp.Get("https://api.github.com/repos/cloudflare/cloudflared/releases/latest")
	if err != nil {
		return nil, fmt.Errorf("请求 GitHub API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, fmt.Errorf("解析 release 信息失败: %w", err)
	}

	return &release, nil
}

// startDownloadCloudflared 启动异步下载 cloudflared 二进制文件
// 通过 DownloadFile Host Function 将下载任务交给宿主端执行
func startDownloadCloudflared(ctx context.Context, platform string, pluginID int64) (*downloadStartResult, error) {
	mapping, ok := platformMapping[platform]
	if !ok {
		return nil, fmt.Errorf("不支持的平台: %s", platform)
	}

	// 获取最新 release
	release, err := getLatestRelease()
	if err != nil {
		return nil, err
	}

	// 查找对应的 asset
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == mapping.FileName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return nil, fmt.Errorf("未找到平台 %s 对应的下载文件 %s", platform, mapping.FileName)
	}

	slog.Info("启动异步下载 cloudflared", "url", downloadURL, "platform", platform)

	// 确定目标文件名和路径（WASM 视角）
	targetName := "cloudflared"
	if strings.Contains(platform, "windows") {
		targetName = "cloudflared.exe"
	}
	destPath := "/cloudflared/bin/" + targetName

	// 调用 DownloadFile Host Function
	hostFunctions := pbplugin.NewHostFunctions()
	resp, err := hostFunctions.DownloadFile(ctx, &pbplugin.DownloadFileRequest{
		Url:               downloadURL,
		DestPath:          destPath,
		TaskId:            downloadTaskID,
		PluginId:          pluginID,
		ExtractTgz:        mapping.NeedsExtract,
		ExtractTargetName: targetName,
	})
	if err != nil {
		return nil, fmt.Errorf("调用 DownloadFile 失败: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("启动下载失败: %s", resp.Message)
	}

	return &downloadStartResult{
		Success:  true,
		Message:  "下载任务已启动",
		TaskID:   resp.TaskId,
		Version:  release.TagName,
		Platform: platform,
	}, nil
}
