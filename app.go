package main

import (
	"context"
	"fetch_reel/engine"
	"fetch_reel/engine/downloader"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	hook "github.com/robotn/gohook"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	manager    *engine.Manager
	sniffer    *engine.Sniffer
	downloader *downloader.Downloader
	env        *engine.EnvResolver

	// 窗口状态
	isHidden   bool
	isExpanded bool
}

func NewApp() *App {
	env := engine.NewEnvResolver()
	manager := engine.NewManager()
	sniffer := engine.NewSniffer(manager, env)
	dl := downloader.NewDownloader(manager, env)

	return &App{
		manager:    manager,
		sniffer:    sniffer,
		downloader: dl,
		env:        env,
		isHidden:   false,
		isExpanded: false,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.SetContext(ctx)

	// 启动时初始化：靠右侧置顶
	// runtime.WindowSetAlwaysOnTop(a.ctx, true)

	// 注册全局热键 Ctrl + Alt + X
	// go a.setupGlobalHotkeys()
}

// setupGlobalHotkeys 监听系统全局按键
func (a *App) setupGlobalHotkeys() {
	// 1. 注册组合键逻辑
	// 注意：gohook 的组合键顺序通常不敏感，但建议写全
	hook.Register(hook.KeyDown, []string{"ctrl", "alt", "x"}, func(e hook.Event) {
		// 当按下组合键时触发
		a.ToggleWindow()
	})

	// 2. 启动监听
	s := hook.Start()

	// 3. 进入事件循环 (Process 会阻塞当前协程)
	<-hook.Process(s)
}

// ToggleWindow 切换窗口的显示和隐藏
func (a *App) ToggleWindow() {
	if a.isHidden {
		runtime.WindowShow(a.ctx)
		runtime.WindowSetAlwaysOnTop(a.ctx, true)
		a.isHidden = false
	} else {
		runtime.WindowHide(a.ctx)
		a.isHidden = true
	}
}

// SetExpanded 切换侧边栏宽度 (供前端裁切功能调用)
// expand: true -> 800px, false -> 350px
func (a *App) SetExpanded(expand bool) {
	a.isExpanded = expand
	if expand {
		runtime.WindowSetSize(a.ctx, 800, 768)
	} else {
		runtime.WindowSetSize(a.ctx, 350, 768)
	}
}

// StartBrowser 启动浏览器
func (a *App) StartBrowser() string {
	exePath, _ := os.Executable()
	userDataDir := filepath.Join(filepath.Dir(exePath), "chrome_data")

	err := a.sniffer.StartBrowser(userDataDir)
	if err != nil {
		return fmt.Sprintf("错误: %v", err)
	}
	return "浏览器启动成功"
}

// OpenDownloadFolder 打开下载目录
func (a *App) OpenDownloadFolder() {
	exePath, _ := os.Executable()
	downloadDir := filepath.Join(filepath.Dir(exePath), "Downloads")
	_ = os.MkdirAll(downloadDir, 0755)

	// 使用 Wails 的 BrowserOpenURL 打开本地路径
	runtime.BrowserOpenURL(a.ctx, downloadDir)
}

// ... 之前的 CreateDownloadTask, StartDownload, StopDownload 逻辑保持不变 ...
// 记得在 CreateDownloadTask 中使用 a.manager.AddTask(task)
// CreateDownloadTask 创建新下载任务
func (a *App) CreateDownloadTask(sniffEvent engine.SniffEvent) (*engine.VideoTask, error) {
	// 1. 预检请求：获取文件大小并检查是否支持 Range
	size, supportRange := a.preCheckResource(sniffEvent.Url, sniffEvent.Headers)

	// 2. 准备路径
	exePath, _ := os.Executable()
	downloadDir := filepath.Join(filepath.Dir(exePath), "Downloads")
	_ = os.MkdirAll(downloadDir, 0755)

	taskID := uuid.New().String()
	tempDir := filepath.Join(downloadDir, ".temp", taskID)

	safeTitle := a.sanitizeFilename(sniffEvent.Title)
	savePath := filepath.Join(downloadDir, safeTitle+".mp4")

	task := &engine.VideoTask{
		ID:           taskID,
		Title:        safeTitle,
		Url:          sniffEvent.Url,
		OriginUrl:    sniffEvent.OriginUrl,
		TargetID:     sniffEvent.TargetID,
		Type:         sniffEvent.Type,
		Status:       "sniffed",
		Size:         size,
		SupportRange: supportRange,
		Headers:      sniffEvent.Headers,
		SavePath:     savePath,
		TempDir:      tempDir,
		Clips:        []engine.TimeRange{},
	}

	a.manager.AddTask(task)
	return task, nil
}

// UpdateTaskUrl 重绑定功能：当链接失效时，更新现有任务的链接
func (a *App) UpdateTaskUrl(taskID string, newUrl string, newHeaders map[string]string) string {
	task := a.manager.GetTaskByID(taskID)
	if task == nil {
		return "任务不存在"
	}

	// 更新链接和请求头
	task.Url = newUrl
	task.Headers = newHeaders

	// 如果是 MP4，重新预检一次（万一 CDN 切换了 Range 支持）
	if task.Type == "mp4" {
		size, support := a.preCheckResource(newUrl, newHeaders)
		task.Size = size
		task.SupportRange = support
	}

	a.manager.AddTask(task) // 触发持久化保存
	return "链接更新成功，可继续下载"
}

// StartDownload 启动下载
func (a *App) StartDownload(taskID string) {
	a.downloader.Start(taskID)
}

// StopDownload 暂停下载
func (a *App) StopDownload(taskID string) {
	a.downloader.Stop(taskID)
}

// preCheckResource 预检资源信息
func (a *App) preCheckResource(url string, headers map[string]string) (int64, bool) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return 0, false
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()

	size := resp.ContentLength
	// 检查服务器是否返回了 Accept-Ranges: bytes
	supportRange := strings.Contains(resp.Header.Get("Accept-Ranges"), "bytes")

	return size, supportRange
}

func (a *App) sanitizeFilename(name string) string {
	badChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	result := name
	for _, char := range badChars {
		result = strings.ReplaceAll(result, char, "_")
	}
	return strings.TrimSpace(result)
}

func (a *App) GetTasks() []*engine.VideoTask {
	return a.manager.GetAllTasks()
}

// UpdateTaskClips 更新任务的剪辑区间
func (a *App) UpdateTaskClips(taskID string, clips []engine.TimeRange) {
	task := a.manager.GetTaskByID(taskID)
	if task != nil {
		task.Clips = clips
		a.manager.AddTask(task) // 存盘
	}
}

// DeleteTask 删除任务
func (a *App) DeleteTask(taskID string) {
	// 1. 停止下载
	a.downloader.Stop(taskID)

	task := a.manager.GetTaskByID(taskID)
	if task != nil {
		// 2. 清理临时目录
		_ = os.RemoveAll(task.TempDir)
		// 3. 从管理器移除
		a.manager.RemoveTask(taskID)
	}
}
