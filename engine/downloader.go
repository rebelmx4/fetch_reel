package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Downloader struct {
	manager *Manager
	active  sync.Map
}

func NewDownloader(m *Manager) *Downloader {
	return &Downloader{manager: m}
}

func (d *Downloader) Start(taskID string, ffmpegPath string) {
	task := d.manager.GetTaskByID(taskID)
	if task == nil {
		return
	}

	// 确认 models.go 中 TempDir 是大写开头的导出版
	if err := os.MkdirAll(task.TempDir, 0755); err != nil {
		d.manager.UpdateTaskStatus(taskID, "error")
		return
	}

	go d.executeDownload(task, ffmpegPath)
}

func (d *Downloader) executeDownload(task *VideoTask, ffmpegPath string) {
	d.manager.UpdateTaskStatus(task.ID, "downloading")

	// 情况 A: 没有任何标记数据 -> 默认下载全片
	if len(task.Clips) == 0 {
		err := d.runFFmpegDownload(task, ffmpegPath, task.Url, task.SavePath, 0, 0)
		if err == nil {
			d.manager.UpdateTaskStatus(task.ID, "done")
		} else {
			d.manager.UpdateTaskStatus(task.ID, "error")
		}
		return
	}

	// 情况 B: 只有一个片段 -> 直接下载这一段到最终路径 (不需要合并)
	if len(task.Clips) == 1 {
		clip := task.Clips[0]
		// 如果状态是 exclude，那就不下载，直接结束（虽然逻辑上不应该出现这种情况）
		if clip.Status != "keep" {
			d.manager.UpdateTaskStatus(task.ID, "done")
			return
		}

		// 关键修正：这里传入 clip.Start 和 clip.End
		err := d.runFFmpegDownload(task, ffmpegPath, task.Url, task.SavePath, clip.Start, clip.End)
		if err == nil {
			d.manager.UpdateTaskStatus(task.ID, "done")
		} else {
			d.manager.UpdateTaskStatus(task.ID, "error")
		}
		return
	}

	// 情况 C: 多个片段 -> 分段下载到临时目录，最后合并
	d.processClips(task, ffmpegPath)
}

func (d *Downloader) processClips(task *VideoTask, ffmpegPath string) {
	var keptFiles []string

	for i, clip := range task.Clips {
		if clip.Status != "keep" {
			continue
		}

		tempFile := filepath.Join(task.TempDir, fmt.Sprintf("clip_%d.mp4", i))

		if _, err := os.Stat(tempFile); err == nil {
			keptFiles = append(keptFiles, tempFile)
			continue
		}

		err := d.runFFmpegDownload(task, ffmpegPath, task.Url, tempFile, clip.Start, clip.End)
		if err == nil {
			keptFiles = append(keptFiles, tempFile)
		}
	}

	if len(keptFiles) > 0 {
		d.mergeClips(task, ffmpegPath, keptFiles)
	}
}

func (d *Downloader) runFFmpegDownload(task *VideoTask, ffmpeg string, url, output string, ss, to float64) error {
	// 1. 基础参数
	// -hide_banner: 不显示 FFmpeg 启动时的版权信息，让日志干净点
	// -y: 如果输出文件已存在，直接覆盖，不询问
	args := []string{"-hide_banner", "-y"}

	// 2. 注入 Headers (解决防盗链的关键)
	// 我们在前面 sniffer_specific.go 里抓到的 Referer, Cookie 等都在 task.Headers 里
	// 这里的 -headers 选项会让 FFmpeg 在发请求时带上这些头
	if len(task.Headers) > 0 {
		var sb strings.Builder
		for k, v := range task.Headers {
			sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
		// 注意：这个参数必须放在 -i (输入) 之前，否则不生效
		args = append(args, "-headers", sb.String())
	}

	// 3. 时间裁切 (你的核心功能)
	// 如果传入了结束时间 (to > 0)，说明是截取
	if to > 0 {
		// -ss: 开始时间 (Start Seek)
		// -to: 结束时间
		// 把它们放在 -i 之前（Input Seeking），FFmpeg 会利用关键帧索引快速跳转
		// 速度极快，且不会黑屏
		args = append(args, "-ss", fmt.Sprintf("%.3f", ss), "-to", fmt.Sprintf("%.3f", to))
	}

	// 4. 输入输出配置
	// -i url: 视频地址 (可以是 http://...mp4 也可以是 http://...m3u8)
	// -c copy: "流复制"模式。直接把视频/音频数据拷贝过去，**不进行重新编码**。
	//          这保证了下载速度=网速，且画质无损。
	// -avoid_negative_ts make_zero: 这是一个高级参数。
	//          当你剪切视频中间一段时，时间戳可能不是从0开始的。
	//          这个参数强制把剪出来的视频时间戳归零，防止播放器打开时卡顿或进度条错乱。
	args = append(args, "-i", url, "-c", "copy", "-avoid_negative_ts", "make_zero", output)

	// 5. 启动进程
	cmd := exec.Command(ffmpeg, args...)

	d.active.Store(task.ID, cmd)
	defer d.active.Delete(task.ID)

	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderr)

	// 改进正则表达式：匹配 size=... 和 bitrate=... (作为速度) 或直接匹配 speed=...
	// 例子: size=    1024kB time=00:00:05.00 bitrate=1638.4kbits/s speed=4.5x
	reSize := regexp.MustCompile(`size=\s*(\d+)kB`)
	reSpeed := regexp.MustCompile(`bitrate=\s*([\d.]+\w+/s)`) // 使用比特率作为瞬时速度
	// 如果 FFmpeg 版本支持，也可以直接抓 speed=...x
	reXSpeed := regexp.MustCompile(`speed=\s*([\d.]+x)`)

	for scanner.Scan() {
		line := scanner.Text()

		var currentSize int64 = 0
		var currentSpeed string = "0 KB/s"

		// 1. 解析已下载大小 (KB -> Bytes)
		if match := reSize.FindStringSubmatch(line); len(match) > 1 {
			kb, _ := strconv.ParseInt(match[1], 10, 64)
			currentSize = kb * 1024
		}

		// 2. 解析速度
		if match := reSpeed.FindStringSubmatch(line); len(match) > 1 {
			currentSpeed = match[1]
		} else if match := reXSpeed.FindStringSubmatch(line); len(match) > 1 {
			currentSpeed = match[1] // 例如 "5.2x"
		}

		// 3. 【关键调用】更新 Manager 状态
		// 这样 React 监听的任务列表就会实时变动
		if currentSize > 0 {
			d.manager.UpdateTaskProgress(task.ID, currentSize, currentSpeed)
		}
	}

	return cmd.Wait()
}

func (d *Downloader) mergeClips(task *VideoTask, ffmpeg string, files []string) {
	d.manager.UpdateTaskStatus(task.ID, "merging")

	listPath := filepath.Join(task.TempDir, "concat.txt")
	var sb strings.Builder
	for _, f := range files {
		absPath, _ := filepath.Abs(f)
		sb.WriteString(fmt.Sprintf("file '%s'\n", strings.ReplaceAll(absPath, "\\", "/")))
	}
	_ = os.WriteFile(listPath, []byte(sb.String()), 0644)

	args := []string{"-y", "-f", "concat", "-safe", "0", "-i", listPath, "-c", "copy", task.SavePath}
	cmd := exec.Command(ffmpeg, args...)
	if err := cmd.Run(); err == nil {
		d.manager.UpdateTaskStatus(task.ID, "done")
	} else {
		d.manager.UpdateTaskStatus(task.ID, "error")
	}
}

func (d *Downloader) Stop(taskID string) {
	if cmd, ok := d.active.Load(taskID); ok {
		_ = cmd.(*exec.Cmd).Process.Kill()
	}
}
