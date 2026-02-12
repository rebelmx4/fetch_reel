package main

import (
	"context"
	"fetch_reel/engine" // 请确保这里的路径与 go.mod 一致
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// App struct
type App struct {
	ctx        context.Context
	manager    *engine.Manager
	sniffer    *engine.Sniffer
	downloader *engine.Downloader
	proxy      *engine.ProxyServer
	hlsParser  *engine.HLSParser
	httpClient *http.Client
}

// NewApp creates a new App application struct
func NewApp() *App {
	manager := engine.NewManager()
	return &App{
		manager:    manager,
		sniffer:    engine.NewSniffer(manager),
		downloader: engine.NewDownloader(manager),
		proxy:      engine.NewProxyServer(12345),
		hlsParser:  &engine.HLSParser{},
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // 设置 10 秒超时
		},
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.SetContext(ctx)
	a.proxy.Start()
}

// StartBrowser 启动浏览器
func (a *App) StartBrowser() string {
	exePath, _ := os.Executable()
	baseDir := filepath.Dir(exePath)

	// 修正：在本地开发时，可能需要指向真实的 Edge 路径或 bin 目录
	chromePath := filepath.Join(baseDir, "bin", "msedge.exe")
	userDataDir := filepath.Join(baseDir, "edge_data")

	err := a.sniffer.StartBrowser(chromePath, userDataDir)
	if err != nil {
		return fmt.Sprintf("错误: %v", err)
	}
	return "浏览器启动成功"
}

// CreateDownloadTask 创建任务（处理最高清晰度选择）
func (a *App) CreateDownloadTask(sniffedUrl, title, originUrl, videoType string, headers map[string]string) (*engine.VideoTask, error) {
	finalUrl := sniffedUrl
	if videoType == "hls" {
		bestUrl, err := a.hlsParser.GetHighestQualityURL(sniffedUrl)
		if err == nil {
			finalUrl = bestUrl
		}
	}

	// === 新增：预检请求，获取文件总大小 ===
	var totalSize int64 = 0
	if videoType == "mp4" {
		req, err := http.NewRequest("HEAD", finalUrl, nil)
		if err == nil {
			// 如果有自定义 Header (如 Cookie)，带上
			for k, v := range headers {
				req.Header.Set(k, v)
			}
			resp, err := a.httpClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					totalSize = resp.ContentLength
				}
			}
		}
	}
	// ===================================

	exePath, _ := os.Executable()
	baseDir := filepath.Dir(exePath)
	downloadDir := filepath.Join(baseDir, "Downloads")
	tempDir := filepath.Join(downloadDir, ".temp", uuid.New().String())

	safeTitle := sanitizeFilename(title)
	savePath := filepath.Join(downloadDir, safeTitle+".mp4")

	task := &engine.VideoTask{
		ID:        uuid.New().String(),
		Title:     safeTitle,
		Url:       finalUrl,
		OriginUrl: originUrl,
		Type:      videoType,
		Status:    "sniffed",
		Size:      totalSize,
		Headers:   headers, // 把 Header 存入任务
		SavePath:  savePath,
		TempDir:   tempDir,
		Clips:     []engine.Clip{},
	}

	a.manager.AddTask(task)
	return task, nil
}

// StartDownload 执行下载
func (a *App) StartDownload(taskID string) string {
	exePath, _ := os.Executable()
	ffmpegPath := filepath.Join(filepath.Dir(exePath), "bin", "ffmpeg.exe")

	a.downloader.Start(taskID, ffmpegPath)
	return "已加入下载队列"
}

// StopDownload 停止下载
func (a *App) StopDownload(taskID string) {
	a.downloader.Stop(taskID)
	a.manager.UpdateTaskStatus(taskID, "paused")
}

// GetTasks 获取所有任务
func (a *App) GetTasks() []*engine.VideoTask {
	return a.manager.GetAllTasks()
}

// 修复后的辅助函数：真正清理非法文件名
func sanitizeFilename(name string) string {
	// 定义 Windows 不允许的文件名字符
	badChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}

	result := name
	for _, char := range badChars {
		// 将非法字符替换为下划线
		result = strings.ReplaceAll(result, char, "_")
	}

	// 去除首尾空格
	result = strings.TrimSpace(result)

	if result == "" {
		return "video_" + uuid.New().String()[:8]
	}
	return result
}

func (a *App) GetSniffEventModel() engine.SniffEvent {
	return engine.SniffEvent{}
}
