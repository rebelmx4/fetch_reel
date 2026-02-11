package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// Sniffer 负责控制外部 Chrome 并拦截视频地址
type Sniffer struct {
	manager *Manager
	cancel  context.CancelFunc // 用于停止嗅探逻辑
}

func GetSystemEdgePath() string {
	// 标准安装路径
	path := `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
	if _, err := os.Stat(path); err == nil {
		return path
	}
	// 备选路径
	path = `C:\Program Files\Microsoft\Edge\Application\msedge.exe`
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func NewSniffer(m *Manager) *Sniffer {
	return &Sniffer{manager: m}
}

// StartBrowser 启动绿色版 Chrome 并开启调试端口
// chromePath: 绿色版 chrome.exe 的路径
// userDataDir: 独立的用户数据目录
func (s *Sniffer) StartBrowser(chromePath string, userDataDir string) error {
	// 1. 构造启动参数
	// --remote-debugging-port=9222 是 CDP 嗅探的关键
	args := []string{
		"--remote-debugging-port=9222",
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		"--no-first-run",
		"--no-default-browser-check",
	}

	cmd := exec.Command(chromePath, args...)

	// 启动浏览器（非阻塞）
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("启动 Chrome 失败: %v", err)
	}

	// 2. 等待浏览器启动并连接 CDP
	// 给浏览器一点启动时间
	time.Sleep(2 * time.Second)

	go s.listenToCDP()

	return nil
}

// listenToCDP 连接到 Chrome 的调试端口并开始嗅探
func (s *Sniffer) listenToCDP() {
	// 连接到现有的 Chrome 实例 (9222 端口)
	allocatorContext, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9222/")
	s.cancel = cancel

	ctx, _ := chromedp.NewContext(allocatorContext)

	// 设置监听器
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			// 过滤视频链接
			url := e.Request.URL
			if s.isMediaURL(url) {
				// 嗅探到了！
				log.Printf("嗅探到视频: %s", url)

				// 构造事件发给前端
				// 注意：这里暂时拿不到标题，后续可以通过 Page.getTitle 获取
				event := &SniffEvent{
					Url:       url,
					Type:      s.getURLType(url),
					OriginUrl: e.DocumentURL,
				}

				// 通过管家发送事件通知 React
				s.manager.emitEvent("video_sniffed", event)
			}
		}
	})

	// 启动 CDP 运行循环
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		log.Printf("CDP 运行出错: %v", err)
	}
}

// isMediaURL 判断是否为我们要找的视频资源
func (s *Sniffer) isMediaURL(url string) bool {
	lowerURL := strings.ToLower(url)
	return strings.Contains(lowerURL, ".m3u8") ||
		strings.Contains(lowerURL, ".mp4") ||
		strings.Contains(lowerURL, "/hls/")
}

// getURLType 识别是 hls 还是 mp4
func (s *Sniffer) getURLType(url string) string {
	if strings.Contains(strings.ToLower(url), ".m3u8") || strings.Contains(url, "/hls/") {
		return "hls"
	}
	return "mp4"
}

// AnalyzeSpecificSite 针对特定域名执行自定义逻辑
func (s *Sniffer) AnalyzeSpecificSite(ctx context.Context, url string) {
	if strings.Contains(url, "bilibili.com") {
		var videoUrl string
		// 这里的 Evaluate 就像在浏览器控制台输入 JS 一样
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.__playinfo__.data.dash.video[0].baseUrl`, &videoUrl),
		)
		if err == nil && videoUrl != "" {
			log.Printf("B站精准嗅探成功: %s", videoUrl)
			// 发送给前端...
		}
	}
}

// Stop 停止嗅探
func (s *Sniffer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}
