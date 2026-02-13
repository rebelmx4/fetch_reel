package engine

// TimeRange 代表用户保留的视频时间区间
type TimeRange struct {
	Index int     `json:"index"`
	Start float64 `json:"start"` // 起始时间（秒）
	End   float64 `json:"end"`   // 结束时间（秒）
}

// VideoTask 代表一个完整的视频下载/处理任务
type VideoTask struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	Url          string            `json:"url"`       // 当前有效的下载链接
	OriginUrl    string            `json:"originUrl"` // 原始网页地址，用于失效时重新嗅探
	TargetID     string            `json:"targetId"`  // 来源标签页 ID
	Type         string            `json:"type"`      // "mp4" 或 "hls"
	Status       string            `json:"status"`    // "sniffed", "downloading", "paused", "merging", "done", "error"
	Size         int64             `json:"size"`      // 文件总大小（字节）
	Downloaded   int64             `json:"downloaded"`
	SupportRange bool              `json:"supportRange"` // 是否支持 Range 请求
	Progress     float64           `json:"progress"`     // 下载百分比 (0-100)
	Speed        string            `json:"speed"`        // 实时下载速度
	SavePath     string            `json:"savePath"`     // 最终保存的完整路径
	TempDir      string            `json:"tempDir"`      // 临时分段存放目录
	Headers      map[string]string `json:"headers"`      // 下载所需的请求头 (Cookie, Referer等)
	Clips        []TimeRange       `json:"clips"`        // 用户选中的时间片段区间

	// InternalState 存储下载进度的详细状态，用于断点续传
	// 根据 Type 的不同，会存储不同的数据结构
	InternalState *TaskInternalState `json:"internalState"`
}

// TaskInternalState 内部进度详情
type TaskInternalState struct {
	// MP4 专用：存储 50MB 分块的状态
	MP4Chunks []MP4ChunkState `json:"mp4Chunks,omitempty"`
	// HLS 专用：存储 TS 片段的状态
	HLSSegments []HLSSegmentState `json:"hlsSegments,omitempty"`
}

// MP4ChunkState MP4 分块进度
type MP4ChunkState struct {
	Index      int   `json:"index"`
	Start      int64 `json:"start"`
	End        int64 `json:"end"`
	IsFinished bool  `json:"isFinished"`
}

// HLSSegmentState HLS TS 片段进度
type HLSSegmentState struct {
	Index      int    `json:"index"`
	URL        string `json:"url"`
	IsFinished bool   `json:"isFinished"`
}

// SniffEvent 嗅探到新资源时发送给前端的事件
type SniffEvent struct {
	Url          string            `json:"url"`
	Title        string            `json:"title"`
	OriginUrl    string            `json:"originUrl"`
	TargetID     string            `json:"targetId"` // 标识资源来自哪个标签页
	Type         string            `json:"type"`     // "mp4" 或 "hls"
	Size         int64             `json:"size"`
	SupportRange bool              `json:"supportRange"`
	Headers      map[string]string `json:"headers"`
}
