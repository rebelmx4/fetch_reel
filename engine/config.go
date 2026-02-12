package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SniffRule 定义单个嗅探规则
type SniffRule struct {
	Name           string   `json:"name"`
	HostKeyword    string   `json:"host_keyword"`    // URL 必须包含的域名片段 (如 "yyyyy")
	TargetReferer  string   `json:"target_referer"`  // 可选：来源页面必须包含的关键词
	MustContain    string   `json:"must_contain"`    // 可选：URL 必须包含的后缀或路径 (如 ".mp4")
	UrlRegex       string   `json:"url_regex"`       // 新增：支持正则表达式
	CaptureHeaders []string `json:"capture_headers"` // 需要抓取的 Header 列表
}

// ConfigLoader 负责加载规则
func LoadSniffRules() []SniffRule {
	// 1. 获取可执行文件目录
	exePath, err := os.Executable()
	if err != nil {
		return []SniffRule{}
	}
	baseDir := filepath.Dir(exePath)
	configPath := filepath.Join(baseDir, "sniff_rules.json")

	// 开发环境回退：如果找不到，尝试在当前工作目录找
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "sniff_rules.json"
	}

	// 2. 读取文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		// 如果没有配置文件，静默返回空，使用通用嗅探
		return []SniffRule{}
	}

	// 3. 解析 JSON
	var rules []SniffRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return []SniffRule{}
	}

	return rules
}
