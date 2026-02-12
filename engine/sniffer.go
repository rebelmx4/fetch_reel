package engine

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

type Sniffer struct {
	manager         *Manager
	cancel          context.CancelFunc
	rules           []SniffRule // 存储加载的规则
	attachedTargets sync.Map
}

// NewSniffer 初始化时加载配置
func NewSniffer(m *Manager) *Sniffer {
	return &Sniffer{
		manager: m,
		rules:   LoadSniffRules(), // 加载 rules.json
	}
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
		"--disable-infobars",
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

func (s *Sniffer) listenToCDP() {
	// 1. 创建远程分配器
	allocatorContext, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9222/")
	s.cancel = cancel

	// 2. 创建一个根上下文
	rootCtx, _ := chromedp.NewContext(allocatorContext)

	// 3. 核心修正：使用 ListenTarget 监听 Target 域的事件
	chromedp.ListenTarget(rootCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		// 当一个新的目标（如标签页）被创建时
		case *target.EventTargetCreated:
			info := ev.TargetInfo
			if info.Type == "page" {
				log.Printf("发现新标签页: %s (ID: %s)", info.URL, info.TargetID)

				// 防止重复附加
				if _, loaded := s.attachedTargets.LoadOrStore(info.TargetID, true); !loaded {
					// 为这个新标签页创建一个子上下文
					childCtx, _ := chromedp.NewContext(rootCtx, chromedp.WithTargetID(info.TargetID))
					// 在后台 goroutine 中为它附加嗅探器
					go s.attachSnifferToContext(childCtx)
				}
			}
		// 当一个目标被销毁时（如标签页关闭）
		case *target.EventTargetDestroyed:
			s.attachedTargets.Delete(ev.TargetID)
			log.Printf("标签页关闭: (ID: %s)", ev.TargetID)
		}
	})

	// 4. 确保 Target 域的事件被发送
	if err := chromedp.Run(rootCtx, target.SetDiscoverTargets(true)); err != nil {
		log.Printf("设置目标发现失败: %v", err)
		return
	}

	// 5. 为已存在的标签页附加嗅探器
	infos, err := chromedp.Targets(rootCtx)
	if err == nil {
		for _, info := range infos {
			if info.Type == "page" {
				if _, loaded := s.attachedTargets.LoadOrStore(info.TargetID, true); !loaded {
					childCtx, _ := chromedp.NewContext(rootCtx, chromedp.WithTargetID(info.TargetID))
					go s.attachSnifferToContext(childCtx)
				}
			}
		}
	}

	log.Println("CDP 嗅探器已启动，正在监听所有标签页...")
}

// attachSnifferToContext (保持不变，但要确保在 goroutine 中运行)
func (s *Sniffer) attachSnifferToContext(ctx context.Context) {
	// 启用网络域
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		// 如果上下文已关闭（例如标签页秒开秒关），这里可能会报错，可以忽略
		if ctx.Err() == nil {
			log.Printf("为新标签页开启网络监听失败: %v", err)
		}
		return
	}

	// 监听网络事件
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		if req, ok := ev.(*network.EventRequestWillBeSent); ok {
			s.processRequest(req.Request.URL, req.DocumentURL, req)
		}
	})
}

// ... processRequest, matchRule, handleSpecificSniff, handleGenericSniff, isGenericMediaURL, getURLType 保持不变 ...
// 为了代码完整性，这里列出 processRequest
func (s *Sniffer) processRequest(url, docUrl string, e *network.EventRequestWillBeSent) {
	if matchedRule := s.matchRule(url, docUrl); matchedRule != nil {
		s.handleSpecificSniff(matchedRule, e)
		return
	}
	if s.isGenericMediaURL(url) {
		s.handleGenericSniff(url, docUrl)
	}
}

// matchRule 升级版：支持正则
func (s *Sniffer) matchRule(url, docUrl string) *SniffRule {
	for i := range s.rules {
		rule := &s.rules[i]

		// 1. Host 检查
		if rule.HostKeyword != "" && !strings.Contains(url, rule.HostKeyword) {
			continue
		}

		// 2. 简单包含检查
		if rule.MustContain != "" && !strings.Contains(url, rule.MustContain) {
			continue
		}

		// 3. Referer 检查
		if rule.TargetReferer != "" && !strings.Contains(docUrl, rule.TargetReferer) {
			continue
		}

		// 4. 正则表达式检查 (核心新增)
		if rule.UrlRegex != "" {
			matched, err := regexp.MatchString(rule.UrlRegex, url)
			if err != nil || !matched {
				continue // 正则不匹配，跳过
			}
		}

		return rule
	}
	return nil
}

// handleSpecificSniff 处理命中规则的视频
func (s *Sniffer) handleSpecificSniff(rule *SniffRule, e *network.EventRequestWillBeSent) {
	log.Printf("[规则命中: %s] %s", rule.Name, e.Request.URL)

	// 提取指定的 Headers
	headers := make(map[string]string)

	// CDP 的 Headers 是 map[string]interface{}
	// 我们遍历规则中要求的 Header，去请求里找
	for _, key := range rule.CaptureHeaders {
		// 尝试直接获取
		if val, ok := e.Request.Headers[key]; ok {
			headers[key] = fmt.Sprintf("%v", val)
		} else {
			// 尝试从混杂大小写中查找 (HTTP头有时候大小写不敏感)
			for k, v := range e.Request.Headers {
				if strings.EqualFold(k, key) {
					headers[key] = fmt.Sprintf("%v", v)
					break
				}
			}
		}
	}

	// 发送事件
	event := &SniffEvent{
		Url:       e.Request.URL,
		Title:     "专用嗅探资源", // 依然建议配合 Page.GetTitle 优化
		OriginUrl: e.DocumentURL,
		Type:      s.getURLType(e.Request.URL), // 复用原来的类型判断
		Headers:   headers,                     // 把抓到的 Token/Cookie 传给下载器
	}
	s.manager.emitEvent("video_sniffed", event)
}

// handleGenericSniff 通用处理逻辑 (原来的逻辑)
func (s *Sniffer) handleGenericSniff(url, origin string) {
	log.Printf("[通用嗅探] %s", url)
	event := &SniffEvent{
		Url:       url,
		Title:     "未知视频",
		OriginUrl: origin,
		Type:      s.getURLType(url),
		Headers:   nil, // 通用模式通常不需要特殊 Header
	}
	s.manager.emitEvent("video_sniffed", event)
}

// isGenericMediaURL 原来的 isMediaURL
func (s *Sniffer) isGenericMediaURL(url string) bool {
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
