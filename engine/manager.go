package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Manager 负责管理所有的视频任务、系统状态及持久化
type Manager struct {
	ctx         context.Context
	tasks       map[string]*VideoTask
	mu          sync.RWMutex
	storagePath string // 持久化文件路径
}

// NewManager 创建管理实例并尝试从磁盘加载现有任务
func NewManager() *Manager {
	// 获取用户数据目录，准备存储 tasks.json
	exePath, _ := os.Executable()
	dataDir := filepath.Dir(exePath)
	storagePath := filepath.Join(dataDir, "tasks.json")

	m := &Manager{
		tasks:       make(map[string]*VideoTask),
		storagePath: storagePath,
	}

	// 初始化时尝试从磁盘读取
	m.loadFromDisk()
	return m
}

func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

// AddTask 添加任务并持久化
func (m *Manager) AddTask(task *VideoTask) {
	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	m.saveToDisk() // 状态变更，保存一次
	m.emitEvent("task_list_updated", m.GetAllTasks())
}

// GetAllTasks 获取列表
func (m *Manager) GetAllTasks() []*VideoTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 修复点：确保初始化为一个空切片而不是 nil
	list := make([]*VideoTask, 0)

	for _, task := range m.tasks {
		list = append(list, task)
	}
	return list
}

// UpdateTaskProgress 更新进度（高频操作，通常不触发磁盘写入，仅内存更新和事件推送）
func (m *Manager) UpdateTaskProgress(id string, downloaded int64, speed string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if task, ok := m.tasks[id]; ok {
		task.Downloaded = downloaded
		task.Speed = speed
		if task.Size > 0 {
			task.Progress = float64(downloaded) / float64(task.Size) * 100
		}
		m.emitEvent("task_progress", task)
	}
}

// saveToDisk 将当前所有任务存入 JSON 文件
func (m *Manager) saveToDisk() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.tasks, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.storagePath, data, 0644)
}

// loadFromDisk 从磁盘读取已保存的任务
func (m *Manager) loadFromDisk() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.storagePath)
	if err != nil {
		return // 文件不存在或读取失败，跳过
	}

	// 将 JSON 重新装载进内存 map
	_ = json.Unmarshal(data, &m.tasks)
}

// UpdateTaskStatus 更新状态并持久化（例如从 "downloading" 变更为 "done"）
func (m *Manager) UpdateTaskStatus(id string, status string) {
	m.mu.Lock()
	if task, ok := m.tasks[id]; ok {
		task.Status = status
	}
	m.mu.Unlock()

	m.saveToDisk()
	m.emitEvent("task_list_updated", m.GetAllTasks())
}

func (m *Manager) emitEvent(eventName string, data interface{}) {
	if m.ctx != nil {
		runtime.EventsEmit(m.ctx, eventName, data)
	}
}

// GetTaskByID 根据 ID 获取单个任务
func (m *Manager) GetTaskByID(id string) *VideoTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[id]
}
