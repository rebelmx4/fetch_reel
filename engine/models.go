package engine

// Clip 代表视频中的一个片段标记
type Clip struct {
	ID     int     `json:"id"`
	Start  float64 `json:"start"`  // 片段起始时间（秒）
	End    float64 `json:"end"`    // 片段结束时间（秒）
	Status string  `json:"status"` // 状态: "keep" (保留) 或 "exclude" (排除)
}

// VideoTask 代表一个完整的视频下载/处理任务
type VideoTask struct {
	ID         string  `json:"id"`         // 任务唯一标识 (UUID)
	Title      string  `json:"title"`      // 网页标题，将作为默认文件名
	Url        string  `json:"url"`        // 当前正在使用的下载链接
	OriginUrl  string  `json:"originUrl"`  // 原始网页地址，用于地址失效时重新嗅探
	Type       string  `json:"type"`       // 资源类型: "mp4" 或 "hls"
	Status     string  `json:"status"`     // 任务状态: "sniffed", "downloading", "merging", "expired", "done"
	Size       int64   `json:"size"`       // 文件总大小（字节）
	Downloaded int64   `json:"downloaded"` // 已下载大小（字节）
	Progress   float64 `json:"progress"`   // 下载百分比 (0-100)
	Speed      string  `json:"speed"`      // 实时下载速度描述 (如 "2.4 MB/s")
	SavePath   string  `json:"savePath"`   // 最终保存的完整路径
	TempDir    string  `json:"tempDir"`    // 临时分段存放目录
	Clips      []Clip  `json:"clips"`      // 用户在轨道上标记的片段集合
}

// SniffEvent 嗅探到新资源时发送给前端的事件结构
type SniffEvent struct {
	Url       string `json:"url"`
	Title     string `json:"title"`
	OriginUrl string `json:"originUrl"`
	Type      string `json:"type"` // "mp4" 或 "hls"
	Size      int64  `json:"size"` // 如果能获取到 Content-Length 则有值
}
