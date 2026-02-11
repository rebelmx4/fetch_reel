package engine

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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

	if len(task.Clips) <= 1 {
		d.runFFmpegDownload(task, ffmpegPath, task.Url, task.SavePath, 0, 0)
	} else {
		d.processClips(task, ffmpegPath)
	}
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
	args := []string{"-hide_banner", "-y"}

	if to > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.3f", ss), "-to", fmt.Sprintf("%.3f", to))
	}

	args = append(args, "-i", url, "-c", "copy", "-avoid_negative_ts", "make_zero", output)

	cmd := exec.Command(ffmpeg, args...)
	d.active.Store(task.ID, cmd)
	defer d.active.Delete(task.ID)

	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stderr)
	re := regexp.MustCompile(`time=(\d{2}:\d{2}:\d{2}.\d{2})`)

	for scanner.Scan() {
		line := scanner.Text()
		if match := re.FindStringSubmatch(line); len(match) > 1 {
			fmt.Printf("任务 %s 进度更新: %s\n", task.ID, match[1])
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
