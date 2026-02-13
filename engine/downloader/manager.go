package downloader

import (
	"context"
	"fetch_reel/engine"
	"os"
	"path/filepath"
	"sync"
)

type Downloader struct {
	manager     *engine.Manager     // 引用全局任务管理器，用于更新 tasks.json
	env         *engine.EnvResolver // 环境探测器
	activeTasks sync.Map            // map[string]context.CancelFunc 存储正在运行的任务
}

func NewDownloader(m *engine.Manager, env *engine.EnvResolver) *Downloader {
	return &Downloader{
		manager: m,
		env:     env,
	}
}

// Start 启动或恢复一个下载任务
func (d *Downloader) Start(taskID string) {
	task := d.manager.GetTaskByID(taskID)
	if task == nil {
		return
	}

	// 1. 如果任务已经在下载，先停止它（处理链接重绑定的情况）
	d.Stop(taskID)

	// 2. 创建任务上下文，用于控制暂停
	ctx, cancel := context.WithCancel(context.Background())
	d.activeTasks.Store(taskID, cancel)

	// 3. 确保临时目录存在
	_ = os.MkdirAll(task.TempDir, 0755)

	// 4. 根据类型分发给不同的处理器
	go func() {
		defer d.activeTasks.Delete(taskID)

		d.manager.UpdateTaskStatus(taskID, "downloading")

		var err error
		if task.Type == "mp4" {
			err = d.processMP4(ctx, task)
		} else if task.Type == "hls" {
			err = d.processHLS(ctx, task)
		}

		// 检查是正常结束还是被用户暂停
		select {
		case <-ctx.Done():
			// 只有在非错误导致结束时，才标记为暂停
			if task.Status != "error" {
				d.manager.UpdateTaskStatus(taskID, "paused")
			}
		default:
			if err != nil {
				d.manager.UpdateTaskStatus(taskID, "error")
			} else {
				// 执行合并逻辑（在具体的处理器里完成下载后调用）
				d.manager.UpdateTaskStatus(taskID, "done")
			}
		}
	}()
}

// Stop 暂停任务
func (d *Downloader) Stop(taskID string) {
	if cancel, ok := d.activeTasks.Load(taskID); ok {
		cancel.(context.CancelFunc)()
		d.activeTasks.Delete(taskID)
	}
}

// RefreshProgress 基于“真理源”计算进度
// 被具体的下载处理器高频调用，但不触发磁盘写入
func (d *Downloader) RefreshProgress(taskID string, speed string) {
	task := d.manager.GetTaskByID(taskID)
	if task == nil {
		return
	}

	// 扫描临时目录下的所有文件，累加大小
	var downloadedSize int64
	_ = filepath.Walk(task.TempDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			downloadedSize += info.Size()
		}
		return nil
	})

	// 调用全局 Manager 更新内存状态和前端事件
	d.manager.UpdateTaskProgress(taskID, downloadedSize, speed)
}

// GetFFmpegPath 从环境探测器获取路径
func (d *Downloader) GetFFmpegPath() string {
	return d.env.GetFFmpegPath()
}

// 具体的处理方法（在后续文件中实现）
// func (d *Downloader) processMP4(ctx context.Context, task *engine.VideoTask) error
// func (d *Downloader) processHLS(ctx context.Context, task *engine.VideoTask) error
