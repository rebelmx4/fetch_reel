package downloader

import (
	"context"
	"fetch_reel/engine"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// processMP4 处理 MP4 类型视频的下载
func (d *Downloader) processMP4(ctx context.Context, task *engine.VideoTask) error {
	// 1. 初始化分片计划（如果 InternalState 为空则是新任务）
	if task.InternalState == nil || len(task.InternalState.MP4Chunks) == 0 {
		d.prepareMP4Chunks(task)
	}

	// 2. 并发控制：最多 3 个协程
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	errChan := make(chan error, 1)

	for i := range task.InternalState.MP4Chunks {
		chunk := &task.InternalState.MP4Chunks[i]
		if chunk.IsFinished {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			return err
		case sem <- struct{}{}: // 占用槽位
			wg.Add(1)
			go func(c *engine.MP4ChunkState) {
				defer wg.Done()
				defer func() { <-sem }() // 释放槽位

				if err := d.downloadMP4Chunk(ctx, task, c); err != nil {
					select {
					case errChan <- err:
					default:
					}
				}
			}(chunk)
		}
	}

	wg.Wait()

	// 再次检查错误通道
	select {
	case err := <-errChan:
		return err
	default:
	}

	// 3. 所有分片完成后，执行合并
	return d.mergeMP4Chunks(task)
}

// prepareMP4Chunks 划分 50MB 分块
func (d *Downloader) prepareMP4Chunks(task *engine.VideoTask) {
	const chunkSize = 50 * 1024 * 1024 // 50MB
	var chunks []engine.MP4ChunkState

	if task.Size <= 0 || !task.SupportRange {
		// 不支持 Range 或大小未知，作为单个块处理
		chunks = append(chunks, engine.MP4ChunkState{
			Index: 0, Start: 0, End: -1, IsFinished: false,
		})
	} else {
		for i := int64(0); i < task.Size; i += chunkSize {
			end := i + chunkSize - 1
			if end >= task.Size {
				end = task.Size - 1
			}
			chunks = append(chunks, engine.MP4ChunkState{
				Index: len(chunks), Start: i, End: end, IsFinished: false,
			})
		}
	}

	task.InternalState = &engine.TaskInternalState{MP4Chunks: chunks}
	d.manager.AddTask(task) // 触发持久化
}

// downloadMP4Chunk 下载具体的单个分片
func (d *Downloader) downloadMP4Chunk(ctx context.Context, task *engine.VideoTask, chunk *engine.MP4ChunkState) error {
	partPath := filepath.Join(task.TempDir, fmt.Sprintf("part_%d.mp4", chunk.Index))

	// 真理源检查：获取本地已下载大小
	var startPos int64 = chunk.Start
	f, _ := os.Stat(partPath)
	if f != nil {
		if chunk.End != -1 && f.Size() >= (chunk.End-chunk.Start+1) {
			chunk.IsFinished = true
			return nil
		}
		startPos += f.Size()
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "GET", task.Url, nil)
	if err != nil {
		return err
	}

	// 设置 Header
	for k, v := range task.Headers {
		req.Header.Set(k, v)
	}
	if task.SupportRange {
		rangeHeader := fmt.Sprintf("bytes=%d-", startPos)
		if chunk.End != -1 {
			rangeHeader += fmt.Sprintf("%d", chunk.End)
		}
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("服务器响应异常: %s", resp.Status)
	}

	// 写入文件（断点续传模式）
	out, err := os.OpenFile(partPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	// 实时进度更新逻辑（不存盘）
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return writeErr
			}
			// 调用真理源进度统计（由 manager.go 提供）
			d.RefreshProgress(task.ID, "计算中...")
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	chunk.IsFinished = true
	d.manager.AddTask(task) // 分片完成，保存一次状态
	return nil
}

// mergeMP4Chunks 物理合并分片
func (d *Downloader) mergeMP4Chunks(task *engine.VideoTask) error {
	d.manager.UpdateTaskStatus(task.ID, "merging")

	// 处理文件名冲突逻辑 (Title (1).mp4)
	finalPath := d.resolveFinalPath(task.SavePath)

	dest, err := os.Create(finalPath)
	if err != nil {
		return err
	}
	defer dest.Close()

	for i := 0; i < len(task.InternalState.MP4Chunks); i++ {
		partPath := filepath.Join(task.TempDir, fmt.Sprintf("part_%d.mp4", i))
		src, err := os.Open(partPath)
		if err != nil {
			return err
		}
		_, err = io.Copy(dest, src)
		src.Close()
		if err != nil {
			return err
		}
	}

	task.SavePath = finalPath      // 更新可能的自动编号路径
	_ = os.RemoveAll(task.TempDir) // 合并成功，清理临时目录
	return nil
}

// resolveFinalPath 实现自动编号 (n)
func (d *Downloader) resolveFinalPath(path string) string {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	newPath := path
	counter := 1

	for {
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			break
		}
		newPath = fmt.Sprintf("%s (%d)%s", base, counter, ext)
		counter++
	}
	return newPath
}
