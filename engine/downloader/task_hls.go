package downloader

import (
	"context"
	"fetch_reel/engine"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/grafov/m3u8"
)

// processHLS 处理 HLS (m3u8) 下载逻辑
func (d *Downloader) processHLS(ctx context.Context, task *engine.VideoTask) error {
	// 1. 解析 m3u8 获取 TS 列表（如果是新任务或重绑链接）
	if task.InternalState == nil || len(task.InternalState.HLSSegments) == 0 {
		if err := d.prepareHLSSegments(ctx, task); err != nil {
			return err
		}
	}

	// 2. 并发下载 TS 分片
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	total := len(task.InternalState.HLSSegments)

	for i := range task.InternalState.HLSSegments {
		seg := &task.InternalState.HLSSegments[i]
		if seg.IsFinished {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return err
		case sem <- struct{}{}:
			wg.Add(1)
			go func(s *engine.HLSSegmentState) {
				defer wg.Done()
				defer func() { <-sem }()

				if err := d.downloadTSSegment(ctx, task, s); err != nil {
					select {
					case errChan <- err:
					default:
					}
				} else {
					// 下载完一个分片，更新进度百分比（基于数量）
					d.updateHLSProgress(task, total)
				}
			}(seg)
		}
	}

	wg.Wait()

	select {
	case err := <-errChan:
		return err
	default:
	}

	// 3. 调用 FFmpeg 合并
	return d.mergeHLSSegments(task)
}

// prepareHLSSegments 请求并解析 m3u8
func (d *Downloader) prepareHLSSegments(ctx context.Context, task *engine.VideoTask) error {
	req, err := http.NewRequestWithContext(ctx, "GET", task.Url, nil)
	if err != nil {
		return err
	}
	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	playlist, listType, err := m3u8.DecodeFrom(resp.Body, true)
	if err != nil {
		return err
	}

	if listType != m3u8.MEDIA {
		return fmt.Errorf("不支持的 m3u8 类型 (可能是 Master Playlist)")
	}

	mediaList := playlist.(*m3u8.MediaPlaylist)
	var segments []engine.HLSSegmentState
	baseURL, _ := url.Parse(task.Url)

	for i, seg := range mediaList.Segments {
		if seg == nil {
			continue
		}
		// 处理相对路径
		u, _ := url.Parse(seg.URI)
		fullURL := baseURL.ResolveReference(u).String()

		segments = append(segments, engine.HLSSegmentState{
			Index:      i,
			URL:        fullURL,
			IsFinished: false,
		})
	}

	task.InternalState = &engine.TaskInternalState{HLSSegments: segments}
	d.manager.AddTask(task)
	return nil
}

// downloadTSSegment 下载单个 TS
func (d *Downloader) downloadTSSegment(ctx context.Context, task *engine.VideoTask, seg *engine.HLSSegmentState) error {
	tsPath := filepath.Join(task.TempDir, fmt.Sprintf("seg_%05d.ts", seg.Index))

	// 真理源检查：如果文件已存在且大小正常，则跳过
	// 注意：由于 TS 很小，我们不处理 TS 内部的断点续传，不完整直接重下
	if f, err := os.Stat(tsPath); err == nil && f.Size() > 0 {
		seg.IsFinished = true
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", seg.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(tsPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	seg.IsFinished = true
	// 每完成一个 TS，不一定非要存盘 tasks.json（太频繁），
	// 我们可以在每完成 10 个分片时存一次，或者在下载循环中处理
	return nil
}

// updateHLSProgress 计算基于数量的进度
func (d *Downloader) updateHLSProgress(task *engine.VideoTask, total int) {
	finished := 0
	for _, s := range task.InternalState.HLSSegments {
		if s.IsFinished {
			finished++
		}
	}

	// 更新内存中的进度百分比
	progress := float64(finished) / float64(total) * 100
	task.Progress = progress

	// 同时触发真理源大小统计和速度计算
	d.RefreshProgress(task.ID, "下载中...")
}

// mergeHLSSegments 使用 FFmpeg 合并 TS
func (d *Downloader) mergeHLSSegments(task *engine.VideoTask) error {
	d.manager.UpdateTaskStatus(task.ID, "merging")

	ffmpegPath := d.GetFFmpegPath()
	if ffmpegPath == "" {
		return fmt.Errorf("找不到 FFmpeg")
	}

	// 1. 生成 concat.txt
	listPath := filepath.Join(task.TempDir, "concat.txt")
	var sb strings.Builder
	for i := 0; i < len(task.InternalState.HLSSegments); i++ {
		// 必须使用绝对路径且处理转义
		tsName := fmt.Sprintf("seg_%05d.ts", i)
		sb.WriteString(fmt.Sprintf("file '%s'\n", tsName))
	}
	_ = os.WriteFile(listPath, []byte(sb.String()), 0644)

	// 2. 自动重名处理
	finalPath := d.resolveFinalPath(task.SavePath)

	// 3. 执行 FFmpeg (隐藏窗口)
	args := []string{
		"-y", "-f", "concat", "-safe", "0", "-i", "concat.txt",
		"-c", "copy", "-avoid_negative_ts", "make_zero", finalPath,
	}

	cmd := exec.Command(ffmpegPath, args...)
	cmd.Dir = task.TempDir // 在临时目录执行，简化 concat.txt 里的路径

	// Windows 隐藏窗口关键设置
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("FFmpeg 合并失败: %v", err)
	}

	task.SavePath = finalPath
	_ = os.RemoveAll(task.TempDir)
	return nil
}
