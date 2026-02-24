package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// taskSnap 存储内存中的实时统计快照
type taskSnap struct {
	lastBytes int64
	lastTime  time.Time
	currBps   float64 // 平滑后的每秒字节数
}

type Manager struct {
	ctx         context.Context
	tasks       map[string]*VideoTask
	stats       map[string]*taskSnap // 任务 ID -> 统计快照
	mu          sync.RWMutex
	storagePath string
}

func NewManager() *Manager {
	exePath, _ := os.Executable()
	storagePath := filepath.Join(filepath.Dir(exePath), "tasks.json")

	m := &Manager{
		tasks:       make(map[string]*VideoTask),
		stats:       make(map[string]*taskSnap),
		storagePath: storagePath,
	}

	m.loadFromDisk()
	return m
}

func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Manager) AddTask(task *VideoTask) {
	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()
	m.saveToDisk()
	m.emitEvent("task_list_updated", m.GetAllTasks())
}

// UpdateTaskProgress 后端核心：计算速度和 ETA
func (m *Manager) UpdateTaskProgress(id string, downloaded int64, _ string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return
	}

	now := time.Now()
	snap, exists := m.stats[id]

	if !exists {
		// 第一次记录
		m.stats[id] = &taskSnap{
			lastBytes: downloaded,
			lastTime:  now,
		}
	} else {
		// 计算增量
		duration := now.Sub(snap.lastTime).Seconds()
		if duration >= 0.5 { // 每 0.5 秒计算一次，避免过于频繁导致读数不稳定
			bytesDiff := downloaded - snap.lastBytes
			instantBps := float64(bytesDiff) / duration

			// 平滑处理速度 (EMA: 指数移动平均)
			if snap.currBps == 0 {
				snap.currBps = instantBps
			} else {
				snap.currBps = snap.currBps*0.7 + instantBps*0.3
			}

			// 更新快照
			snap.lastBytes = downloaded
			snap.lastTime = now

			// 更新任务状态
			task.Speed = m.formatSpeed(snap.currBps)
			if task.Size > 0 && snap.currBps > 0 {
				task.RemainingSeconds = int64(float64(task.Size-downloaded) / snap.currBps)
			} else {
				task.RemainingSeconds = -1 // 未知
			}
		}
	}

	task.Downloaded = downloaded
	if task.Size > 0 {
		task.Progress = float64(downloaded) / float64(task.Size) * 100
	}

	m.emitEvent("task_progress", task)
}

func (m *Manager) formatSpeed(bps float64) string {
	if bps < 1024 {
		return fmt.Sprintf("%.0f B/s", bps)
	} else if bps < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bps/1024)
	}
	return fmt.Sprintf("%.1f MB/s", bps/1024/1024)
}

func (m *Manager) RemoveTask(id string) {
	m.mu.Lock()
	delete(m.tasks, id)
	delete(m.stats, id)
	m.mu.Unlock()
	m.saveToDisk()
	m.emitEvent("task_list_updated", m.GetAllTasks())
}

func (m *Manager) GetAllTasks() []*VideoTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*VideoTask, 0)
	for _, task := range m.tasks {
		list = append(list, task)
	}
	return list
}

func (m *Manager) UpdateTaskStatus(id string, status string) {
	m.mu.Lock()
	if task, ok := m.tasks[id]; ok {
		task.Status = status
		// 如果状态变更为非下载中，清除速度
		if status != "downloading" {
			task.Speed = ""
			task.RemainingSeconds = 0
			delete(m.stats, id)
		}
	}
	m.mu.Unlock()
	m.saveToDisk()
	m.emitEvent("task_list_updated", m.GetAllTasks())
}

func (m *Manager) saveToDisk() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, _ := json.MarshalIndent(m.tasks, "", "  ")
	_ = os.WriteFile(m.storagePath, data, 0644)
}

func (m *Manager) loadFromDisk() {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := os.ReadFile(m.storagePath)
	if err == nil {
		_ = json.Unmarshal(data, &m.tasks)
	}
}

func (m *Manager) GetTaskByID(id string) *VideoTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id]
}

func (m *Manager) emitEvent(eventName string, data interface{}) {
	if m.ctx != nil {
		runtime.EventsEmit(m.ctx, eventName, data)
	}
}
