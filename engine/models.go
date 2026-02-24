package engine

// TimeRange 代表用户保留的视频时间区间
type TimeRange struct {
	Index int     `json:"index"`
	Start float64 `json:"start"` // 起始时间（秒）
	End   float64 `json:"end"`   // 结束时间（秒）
}

// VideoTask 代表一个完整的视频下载/处理任务
type VideoTask struct {
	ID               string            `json:"id"`
	Title            string            `json:"title"`            // 文件展示标题
	Url              string            `json:"url"`              // 当前有效的下载链接
	OriginUrl        string            `json:"originUrl"`        // 原始网页地址
	TargetID         string            `json:"targetId"`         // 来源标签页 ID
	Type             string            `json:"type"`             // "mp4" 或 "hls"
	Status           string            `json:"status"`           // "sniffed", "downloading", "paused", "merging", "done", "error"
	Size             int64             `json:"size"`             // 总大小
	Downloaded       int64             `json:"downloaded"`       // 已下载大小
	Progress         float64           `json:"progress"`         // 百分比
	Speed            string            `json:"speed"`            // 格式化后的速度 (如 "1.2 MB/s")
	RemainingSeconds int64             `json:"remainingSeconds"` // 剩余时间（秒）
	SupportRange     bool              `json:"supportRange"`
	SavePath         string            `json:"savePath"`
	TempDir          string            `json:"tempDir"`
	Headers          map[string]string `json:"headers"`
	Clips            []TimeRange       `json:"clips"`

	InternalState *TaskInternalState `json:"internalState"`
}

type TaskInternalState struct {
	MP4Chunks   []MP4ChunkState   `json:"mp4Chunks,omitempty"`
	HLSSegments []HLSSegmentState `json:"hlsSegments,omitempty"`
}

type MP4ChunkState struct {
	Index      int   `json:"index"`
	Start      int64 `json:"start"`
	End        int64 `json:"end"`
	IsFinished bool  `json:"isFinished"`
}

type HLSSegmentState struct {
	Index      int    `json:"index"`
	URL        string `json:"url"`
	IsFinished bool   `json:"isFinished"`
}

// SniffEvent 嗅探事件数据
type SniffEvent struct {
	Url          string            `json:"url"`
	Title        string            `json:"title"`
	OriginUrl    string            `json:"originUrl"`
	TargetID     string            `json:"targetId"`
	Type         string            `json:"type"`
	Size         int64             `json:"size"`
	SupportRange bool              `json:"supportRange"`
	Headers      map[string]string `json:"headers"`
}
