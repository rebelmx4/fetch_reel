package engine

import (
	"os"
	"path/filepath"
)

// EnvResolver 负责定位系统中的外部资源路径
type EnvResolver struct {
	exeDir string
}

func NewEnvResolver() *EnvResolver {
	exePath, _ := os.Executable()
	return &EnvResolver{
		exeDir: filepath.Dir(exePath),
	}
}

// GetToolPath 获取工具（ffmpeg/chrome）或配置文件（sniff_rules.json）的绝对路径
// toolName: "ffmpeg", "chrome", "config" 等
// fileName: "ffmpeg.exe", "chrome.exe", "sniff_rules.json"
func (e *EnvResolver) GetToolPath(toolName, fileName string) string {
	// 路径 A: {exeDir}/bin/{toolName}/{fileName}
	pathA := filepath.Join(e.exeDir, "bin", toolName, fileName)
	if _, err := os.Stat(pathA); err == nil {
		return pathA
	}

	// 路径 B: ../../runtime_dep/bin/{toolName}/{fileName}
	// 这里的 ../../ 是相对于 exeDir 的开发环境布局
	pathB := filepath.Join(e.exeDir, "..", "..", "runtime_dep", toolName, fileName)
	if _, err := os.Stat(pathB); err == nil {
		return pathB
	}

	return "" // 如果都没找到，返回空，由调用方处理报错
}

// GetFFmpegPath 快捷获取 FFmpeg
func (e *EnvResolver) GetFFmpegPath() string {
	return e.GetToolPath("ffmpeg", "ffmpeg.exe")
}

// GetChromePath 快捷获取 Chrome
func (e *EnvResolver) GetChromePath() string {
	return e.GetToolPath("chrome", "chrome.exe")
}

// GetRulesPath 快捷获取嗅探规则
func (e *EnvResolver) GetRulesPath() string {
	return e.GetToolPath("config", "sniff_rules.json")
}
