//go:build wasip1
// +build wasip1

package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	pluginhttp "github.com/mimusic-org/plugin/pkg/go-plugin-http/http"
)

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

// downloadResult 下载结果
type downloadResult struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
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

// downloadCloudflared 下载 cloudflared 二进制文件
func downloadCloudflared(platform string) (*downloadResult, error) {
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

	slog.Info("开始下载 cloudflared", "url", downloadURL, "platform", platform)

	// 确保 bin 目录存在
	binDir := "/cloudflared/bin"
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 bin 目录失败: %w", err)
	}

	// 下载文件
	resp, err := pluginhttp.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载返回状态码: %d", resp.StatusCode)
	}

	// 确定目标文件名
	targetName := "cloudflared"
	if strings.Contains(platform, "windows") {
		targetName = "cloudflared.exe"
	}
	targetPath := filepath.Join(binDir, targetName)

	if mapping.NeedsExtract {
		// macOS: 需要解压 .tgz 文件
		if err := extractTgz(resp.Body, binDir, targetName); err != nil {
			return nil, fmt.Errorf("解压 tgz 失败: %w", err)
		}
	} else {
		// Linux/Windows: 直接写入二进制文件
		outFile, err := os.Create(targetPath)
		if err != nil {
			return nil, fmt.Errorf("创建文件失败: %w", err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, resp.Body); err != nil {
			return nil, fmt.Errorf("写入文件失败: %w", err)
		}
	}

	// 设置可执行权限（非 Windows）
	if !strings.Contains(platform, "windows") {
		if err := os.Chmod(targetPath, 0755); err != nil {
			slog.Warn("设置可执行权限失败", "error", err)
		}
	}

	slog.Info("cloudflared 下载完成", "path", targetPath, "version", release.TagName)

	return &downloadResult{
		Success:  true,
		Message:  "下载完成",
		Version:  release.TagName,
		Platform: platform,
	}, nil
}

// extractTgz 从 .tgz 文件中提取 cloudflared 可执行文件
func extractTgz(reader io.Reader, destDir string, targetName string) error {
	gzReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("创建 gzip reader 失败: %w", err)
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("读取 tar 条目失败: %w", err)
		}

		// 查找 cloudflared 可执行文件
		baseName := filepath.Base(header.Name)
		if baseName == "cloudflared" && header.Typeflag == tar.TypeReg {
			targetPath := filepath.Join(destDir, targetName)
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("创建目标文件失败: %w", err)
			}
			defer outFile.Close()

			if _, err := io.Copy(outFile, tarReader); err != nil {
				return fmt.Errorf("写入目标文件失败: %w", err)
			}

			slog.Info("已从 tgz 中提取 cloudflared", "path", targetPath)
			return nil
		}
	}

	return fmt.Errorf("tgz 中未找到 cloudflared 可执行文件")
}
