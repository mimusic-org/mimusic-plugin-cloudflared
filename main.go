//go:build wasip1
// +build wasip1

// Package main 实现了 mimusic 系统的 Cloudflared 隧道插件。
// 该插件提供 cloudflared 的下载、更新和运行功能，
// 通过 Cloudflare Tunnel 将本地 MiMusic 服务暴露到公网。
package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/knqyf263/go-plugin/types/known/emptypb"
	"github.com/mimusic-org/plugin/api/pbplugin"
	"github.com/mimusic-org/plugin/api/plugin"
)

// main 函数是 Go 编译为 Wasm 所必需的。
func main() {}

// processID 是 cloudflared 后台进程的标识符
const processID = "cloudflared-tunnel"

// Plugin 实现了 Cloudflared 隧道插件的功能。
type Plugin struct {
	Version  string
	pluginID int64

	staticHandler *plugin.StaticHandler
}

// init 将 Plugin 实现注册到插件框架中。
func init() {
	plugin.RegisterPlugin(&Plugin{
		Version: "1.0.0",
	})
}

// GetPluginInfo 返回此插件的元数据。
func (p *Plugin) GetPluginInfo(ctx context.Context, request *emptypb.Empty) (*pbplugin.GetPluginInfoResponse, error) {
	return &pbplugin.GetPluginInfoResponse{
		Success: true,
		Message: "成功获取插件信息",
		Info: &pbplugin.PluginInfo{
			Name:        "Cloudflared 隧道",
			Version:     p.Version,
			Description: "通过 Cloudflare Tunnel 将本地 MiMusic 服务暴露到公网，支持 cloudflared 的下载和管理",
			Author:      "MiMusic Team",
			Homepage:    "https://github.com/mimusic-org/mimusic",
			EntryPath:   "/cloudflared",
		},
	}, nil
}

//go:embed static/*
var staticFS embed.FS

// Init 在宿主应用程序加载插件时初始化插件。
func (p *Plugin) Init(ctx context.Context, request *pbplugin.InitRequest) (*emptypb.Empty, error) {
	fmt.Println("正在初始化 Cloudflared 隧道插件", p.Version)
	p.pluginID = request.GetPluginId()

	// 获取路由管理器
	rm := plugin.GetRouterManager()

	// 初始化静态文件处理器
	p.staticHandler = plugin.NewStaticHandler(staticFS, "/cloudflared", rm, ctx)

	// 注册 API 路由（需要认证）
	rm.RegisterRouter(ctx, "GET", "/cloudflared/api/status", handleStatus, true)
	rm.RegisterRouter(ctx, "POST", "/cloudflared/api/start", handleStart, true)
	rm.RegisterRouter(ctx, "POST", "/cloudflared/api/stop", handleStop, true)
	rm.RegisterRouter(ctx, "GET", "/cloudflared/api/output", handleOutput, true)
	rm.RegisterRouter(ctx, "POST", "/cloudflared/api/download", handleDownload, true)
	rm.RegisterRouter(ctx, "GET", "/cloudflared/api/download/status", handleDownloadStatus, true)
	rm.RegisterRouter(ctx, "GET", "/cloudflared/api/releases", handleReleases, true)

	slog.Info("Cloudflared 隧道插件路由注册完成", "version", p.Version)
	return &emptypb.Empty{}, nil
}

// Deinit 在宿主应用程序卸载插件时清理资源。
func (p *Plugin) Deinit(ctx context.Context, request *emptypb.Empty) (*emptypb.Empty, error) {
	fmt.Println("正在反初始化 Cloudflared 隧道插件")

	// 尝试停止运行中的 cloudflared 进程
	hostFunctions := pbplugin.NewHostFunctions()
	stopResp, err := hostFunctions.StopCommand(ctx, &pbplugin.StopCommandRequest{
		ProcessId: processID,
		PluginId:  p.pluginID,
	})
	if err != nil {
		slog.Warn("停止 cloudflared 进程失败", "error", err)
	} else if stopResp.Success {
		slog.Info("cloudflared 进程已停止")
	}

	slog.Info("Cloudflared 隧道插件已卸载")
	return &emptypb.Empty{}, nil
}
