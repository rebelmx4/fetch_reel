package main

import (
	"context"
	"fetch_reel/engine"
	"fetch_reel/engine/downloader"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	manager    *engine.Manager
	sniffer    *engine.Sniffer
	downloader *downloader.Downloader
	env        *engine.EnvResolver

	isPinned   bool // 记录是否置顶
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
		isPinned:   true, // 默认置顶（与 main.go 一致）
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.manager.SetContext(ctx)
}

// TogglePin 切换窗口置顶状态
func (a *App) TogglePin() bool {
	a.isPinned = !a.isPinned
	runtime.WindowSetAlwaysOnTop(a.ctx, a.isPinned)
	return a.isPinned
}

// QuitApp 退出程序
func (a *App) QuitApp() {
	runtime.Quit(a.ctx)
}

// SetExpanded 切换宽度 (380 <-> 830)
func (a *App) SetExpanded(expand bool) {
	a.isExpanded = expand
	if expand {
		runtime.WindowSetSize(a.ctx, 830, 720)
	} else {
		runtime.WindowSetSize(a.ctx, 380, 720)
	}
}

// getURLFileName 解析 URL 获取净化后的文件名
// 例子: "abc12345.ts?auth=123" -> "abc12345.ts"
func (a *App) getURLFileName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "video_stream"
	}
	// path.Base 会处理 /path/to/file.mp4 -> file.mp4
	// 同时由于 u.Path 已经不包含 ?query，所以净化自动完成
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		return "video_stream"
	}
	return name
}

func (a *App) CreateDownloadTask(sniffEvent engine.SniffEvent) (*engine.VideoTask, error) {
	// 1. 净化文件名
	cleanName := a.getURLFileName(sniffEvent.Url)

	// 如果 URL 没解析出好名字，才考虑用网页 Title
	finalTitle := cleanName
	if finalTitle == "video_stream" && sniffEvent.Title != "" {
		finalTitle = a.sanitizeFilename(sniffEvent.Title)
	}

	// 2. 预检资源
	finalSize := sniffEvent.Size
	finalSupport := sniffEvent.SupportRange
	if finalSize <= 0 {
		size, support := a.preCheckResource(sniffEvent.Url, sniffEvent.Headers)
		finalSize = size
		if !finalSupport {
			finalSupport = support
		}
	}

	// 3. 准备路径
	exePath, _ := os.Executable()
	downloadDir := filepath.Join(filepath.Dir(exePath), "Downloads")
	_ = os.MkdirAll(downloadDir, 0755)

	taskID := uuid.New().String()
	tempDir := filepath.Join(downloadDir, ".temp", taskID)
	savePath := filepath.Join(downloadDir, finalTitle)
	// 确保有 .mp4 后缀
	if !strings.HasSuffix(strings.ToLower(savePath), ".mp4") && !strings.HasSuffix(strings.ToLower(savePath), ".ts") {
		savePath += ".mp4"
	}

	task := &engine.VideoTask{
		ID:           taskID,
		Title:        finalTitle,
		Url:          sniffEvent.Url,
		OriginUrl:    sniffEvent.OriginUrl,
		TargetID:     sniffEvent.TargetID,
		Type:         sniffEvent.Type,
		Status:       "sniffed",
		Size:         finalSize,
		SupportRange: finalSupport,
		Headers:      sniffEvent.Headers,
		SavePath:     savePath,
		TempDir:      tempDir,
	}

	a.manager.AddTask(task)
	return task, nil
}

// --- 其余方法 (StartDownload, StopDownload, DeleteTask, UpdateTaskUrl 等) 保持原样 ---

func (a *App) StartDownload(taskID string) { a.downloader.Start(taskID) }
func (a *App) StopDownload(taskID string)  { a.downloader.Stop(taskID) }

func (a *App) DeleteTask(taskID string) {
	a.downloader.Stop(taskID)
	task := a.manager.GetTaskByID(taskID)
	if task != nil {
		_ = os.RemoveAll(task.TempDir)
		a.manager.RemoveTask(taskID)
	}
}

func (a *App) UpdateTaskUrl(taskID string, newUrl string, newHeaders map[string]string) string {
	task := a.manager.GetTaskByID(taskID)
	if task == nil {
		return "任务不存在"
	}
	task.Url = newUrl
	task.Headers = newHeaders
	a.manager.AddTask(task)
	return "链接更新成功"
}

func (a *App) UpdateTaskClips(taskID string, clips []engine.TimeRange) {
	task := a.manager.GetTaskByID(taskID)
	if task != nil {
		task.Clips = clips
		a.manager.AddTask(task)
	}
}

func (a *App) GetTasks() []*engine.VideoTask {
	return a.manager.GetAllTasks()
}

func (a *App) StartBrowser() string {
	if err := a.sniffer.StartBrowser(); err != nil {
		return err.Error()
	}
	return "OK"
}

func (a *App) OpenDownloadFolder() {
	exePath, _ := os.Executable()
	dir := filepath.Join(filepath.Dir(exePath), "Downloads")
	_ = os.MkdirAll(dir, 0755)
	runtime.BrowserOpenURL(a.ctx, dir)
}

func (a *App) preCheckResource(url string, headers map[string]string) (int64, bool) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("HEAD", url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	return resp.ContentLength, strings.Contains(strings.ToLower(resp.Header.Get("Accept-Ranges")), "bytes") || resp.StatusCode == 206
}

func (a *App) sanitizeFilename(name string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return strings.TrimSpace(r.Replace(name))
}
