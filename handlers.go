//go:build wasip1
// +build wasip1

package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/mimusic-org/plugin/api/pbplugin"
	"github.com/mimusic-org/plugin/api/plugin"
)

// statusResponse 状态响应
type statusResponse struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Version   string `json:"version,omitempty"`
}

// startRequest 启动请求
type startRequest struct {
	Port string `json:"port"`
}

// downloadRequest 下载请求
type downloadRequest struct {
	Platform string `json:"platform"`
}

// handleStatus 获取 cloudflared 状态
// GET /cloudflared/api/status
func handleStatus(req *http.Request) (*plugin.RouterResponse, error) {
	pluginID := plugin.GetPluginId()

	// 检查 cloudflared 是否已安装
	binPath := "/cloudflared/bin/cloudflared"
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		// Windows 下检查 .exe
		binPath = "/cloudflared/bin/cloudflared.exe"
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			return plugin.SuccessResponse(statusResponse{
				Installed: false,
				Running:   false,
			}), nil
		}
	}

	// 检查是否正在运行
	hostFunctions := pbplugin.NewHostFunctions()
	outputResp, err := hostFunctions.GetCommandOutput(req.Context(), &pbplugin.GetCommandOutputRequest{
		ProcessId: processID,
		PluginId:  pluginID,
	})

	running := false
	if err == nil && outputResp.Success {
		running = outputResp.Running
	}

	// 获取版本信息
	version := getInstalledVersion(req.Context(), pluginID)

	return plugin.SuccessResponse(statusResponse{
		Installed: true,
		Running:   running,
		Version:   version,
	}), nil
}

// getInstalledVersion 获取已安装的 cloudflared 版本
func getInstalledVersion(ctx context.Context, pluginID int64) string {
	hostFunctions := pbplugin.NewHostFunctions()
	resp, err := hostFunctions.ExecuteCommand(ctx, &pbplugin.ExecuteCommandRequest{
		Command:    "cloudflared",
		Args:       []string{"version"},
		PluginId:   pluginID,
		Background: false,
	})
	if err != nil || !resp.Success {
		return ""
	}
	// cloudflared 版本输出在 stdout 或 stderr
	output := resp.Stdout
	if output == "" {
		output = resp.Stderr
	}
	return output
}

// handleStart 启动 cloudflared tunnel
// POST /cloudflared/api/start
func handleStart(req *http.Request) (*plugin.RouterResponse, error) {
	pluginID := plugin.GetPluginId()

	var body startRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return plugin.ErrorResponse(http.StatusBadRequest, "无效的请求体: "+err.Error()), nil
	}

	if body.Port == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "端口号不能为空"), nil
	}

	// 检查是否已在运行
	hostFunctions := pbplugin.NewHostFunctions()
	outputResp, err := hostFunctions.GetCommandOutput(req.Context(), &pbplugin.GetCommandOutputRequest{
		ProcessId: processID,
		PluginId:  pluginID,
	})
	if err == nil && outputResp.Success && outputResp.Running {
		return plugin.ErrorResponse(http.StatusConflict, "cloudflared 已在运行中"), nil
	}

	// 启动 cloudflared tunnel
	resp, err := hostFunctions.ExecuteCommand(req.Context(), &pbplugin.ExecuteCommandRequest{
		Command:    "cloudflared",
		Args:       []string{"tunnel", "--url", "http://localhost:" + body.Port},
		PluginId:   pluginID,
		Background: true,
		ProcessId:  processID,
	})
	if err != nil {
		slog.Error("启动 cloudflared 失败", "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "启动失败: "+err.Error()), nil
	}

	if !resp.Success {
		return plugin.ErrorResponse(http.StatusInternalServerError, "启动失败: "+resp.Message), nil
	}

	slog.Info("cloudflared tunnel 已启动", "port", body.Port)
	return plugin.SuccessResponse(map[string]string{
		"message":    "cloudflared 已启动",
		"process_id": resp.ProcessId,
	}), nil
}

// handleStop 停止 cloudflared
// POST /cloudflared/api/stop
func handleStop(req *http.Request) (*plugin.RouterResponse, error) {
	pluginID := plugin.GetPluginId()

	hostFunctions := pbplugin.NewHostFunctions()
	resp, err := hostFunctions.StopCommand(req.Context(), &pbplugin.StopCommandRequest{
		ProcessId: processID,
		PluginId:  pluginID,
	})
	if err != nil {
		slog.Error("停止 cloudflared 失败", "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "停止失败: "+err.Error()), nil
	}

	if !resp.Success {
		return plugin.ErrorResponse(http.StatusInternalServerError, "停止失败: "+resp.Message), nil
	}

	slog.Info("cloudflared 已停止")
	return plugin.SuccessResponse(map[string]string{
		"message": "cloudflared 已停止",
	}), nil
}

// handleOutput 获取 cloudflared 运行输出
// GET /cloudflared/api/output
func handleOutput(req *http.Request) (*plugin.RouterResponse, error) {
	pluginID := plugin.GetPluginId()

	hostFunctions := pbplugin.NewHostFunctions()
	resp, err := hostFunctions.GetCommandOutput(req.Context(), &pbplugin.GetCommandOutputRequest{
		ProcessId: processID,
		PluginId:  pluginID,
	})
	if err != nil {
		return plugin.ErrorResponse(http.StatusInternalServerError, "获取输出失败: "+err.Error()), nil
	}

	if !resp.Success {
		return plugin.ErrorResponse(http.StatusNotFound, resp.Message), nil
	}

	return plugin.SuccessResponse(map[string]interface{}{
		"stdout":    resp.Stdout,
		"stderr":    resp.Stderr,
		"running":   resp.Running,
		"exit_code": resp.ExitCode,
	}), nil
}

// handleDownload 下载 cloudflared
// POST /cloudflared/api/download
func handleDownload(req *http.Request) (*plugin.RouterResponse, error) {
	var body downloadRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		return plugin.ErrorResponse(http.StatusBadRequest, "无效的请求体: "+err.Error()), nil
	}

	if body.Platform == "" {
		return plugin.ErrorResponse(http.StatusBadRequest, "平台信息不能为空"), nil
	}

	slog.Info("开始下载 cloudflared", "platform", body.Platform)

	result, err := downloadCloudflared(body.Platform)
	if err != nil {
		slog.Error("下载 cloudflared 失败", "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "下载失败: "+err.Error()), nil
	}

	return plugin.SuccessResponse(result), nil
}

// handleReleases 获取 GitHub 最新 release 信息
// GET /cloudflared/api/releases
func handleReleases(req *http.Request) (*plugin.RouterResponse, error) {
	release, err := getLatestRelease()
	if err != nil {
		slog.Error("获取 release 信息失败", "error", err)
		return plugin.ErrorResponse(http.StatusInternalServerError, "获取 release 信息失败: "+err.Error()), nil
	}

	return plugin.SuccessResponse(release), nil
}
