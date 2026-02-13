package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
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

type Sniffer struct {
	manager *Manager
	env     *EnvResolver
	cancel  context.CancelFunc
	rules   []SniffRule
}

func NewSniffer(m *Manager, env *EnvResolver) *Sniffer {
	s := &Sniffer{
		manager: m,
		env:     env,
	}
	s.rules = s.loadRules()
	return s
}

func (s *Sniffer) StartBrowser(userDataDir string) error {
	// 1. 使用 EnvResolver 获取路径
	chromePath := s.env.GetChromePath()
	if chromePath == "" {
		return fmt.Errorf("找不到 Chrome 浏览器，请检查 bin 目录")
	}

	// 2. 构造启动参数 (移除 chromedp 的封装，直接使用 exec)
	args := []string{
		"--remote-debugging-port=9222",
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		"--no-first-run",
		"--no-default-browser-check",
		// 如果需要防止浏览器在后台没关掉，可以加上这个
		"--remote-allow-origins=*",
		"--disable-infobars",
	}

	cmd := exec.Command(chromePath, args...)

	// 3. 启动浏览器进程
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("启动 Chrome 进程失败: %v", err)
	}

	// 4. 等待浏览器完全启动
	time.Sleep(2 * time.Second)

	// 5. 启动 CDP 监听 (保持原来的 listenToCDP 逻辑)
	go s.listenToCDP()

	return nil
}

func (s *Sniffer) listenToCDP() {
	allocatorContext, _ := chromedp.NewRemoteAllocator(context.Background(), "ws://127.0.0.1:9222/")
	rootCtx, _ := chromedp.NewContext(allocatorContext)

	// --- 关键联动逻辑：监听标签页切换和销毁 ---
	chromedp.ListenTarget(rootCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *target.EventTargetCreated:
			if ev.TargetInfo.Type == "page" {
				targetID := ev.TargetInfo.TargetID
				childCtx, _ := chromedp.NewContext(rootCtx, chromedp.WithTargetID(targetID))
				go s.attachSnifferToContext(childCtx, string(targetID))
			}
		case *target.EventTargetInfoChanged:
			// 当用户点击 Chrome 标签页切换焦点时，通知前端
			if ev.TargetInfo.Type == "page" && ev.TargetInfo.Attached {
				s.manager.emitEvent("tab_focused", ev.TargetInfo.TargetID)
			}
		case *target.EventTargetDestroyed:
			// 当标签页关闭时，通知前端清理该标签页的嗅探记录
			s.manager.emitEvent("tab_closed", ev.TargetID)
		}
	})

	chromedp.Run(rootCtx, target.SetDiscoverTargets(true))
}

// 在 sniffer.txt 中修改或添加逻辑

func (s *Sniffer) attachSnifferToContext(ctx context.Context, targetID string) {
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		return
	}

	// 用于暂存请求信息的 Map (Key 是 RequestID)
	// 因为 Request 和 Response 是异步成对出现的
	pendingRequests := make(map[network.RequestID]*SniffEvent)

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {

		// 1. 拦截请求：获取 URL、Referer、Method 等
		case *network.EventRequestWillBeSent:
			url := ev.Request.URL
			docUrl := ev.DocumentURL

			if s.isGenericMediaURL(url) || s.matchRule(url, docUrl) != nil {
				// 先创建一个初步的事件对象，存入 map
				pendingRequests[ev.RequestID] = &SniffEvent{
					Url:       url,
					OriginUrl: docUrl,
					TargetID:  targetID,
					Type:      s.getURLType(url),
					Headers:   s.filterHeaders(ev.Request.Headers, url, docUrl),
				}
			}

		// 2. 拦截响应：判断是否支持 Range，获取文件大小
		case *network.EventResponseReceived:
			if event, ok := pendingRequests[ev.RequestID]; ok {
				// 检查服务器返回的 Header
				headers := ev.Response.Headers

				// 判断 Range 支持
				// 方式 A: 检查 Accept-Ranges 字段
				if val, ok := headers["Accept-Ranges"]; ok && strings.Contains(strings.ToLower(fmt.Sprintf("%v", val)), "bytes") {
					event.SupportRange = true
				}
				// 方式 B: 如果响应状态码直接就是 206 Partial Content
				if ev.Response.Status == 206 {
					event.SupportRange = true
				}

				// 获取文件总大小 (Content-Length)
				if sizeVal, ok := headers["Content-Length"]; ok {
					fmt.Sscanf(fmt.Sprintf("%v", sizeVal), "%d", &event.Size)
				}

				// 延迟获取标题并发送给前端
				time.AfterFunc(800*time.Millisecond, func() {
					var title string
					_ = chromedp.Run(ctx, chromedp.Title(&title))
					if title == "" {
						title = "未知视频"
					}
					event.Title = title

					// 正式上报给前端
					s.manager.emitEvent("video_sniffed", event)
					// 处理完后从暂存区删除
					delete(pendingRequests, ev.RequestID)
				})
			}
		}
	})
}

// 辅助函数：抽取原来的 Header 过滤逻辑
func (s *Sniffer) filterHeaders(cdpHeaders network.Headers, url, docUrl string) map[string]string {
	allHeaders := make(map[string]string)
	for k, v := range cdpHeaders {
		allHeaders[k] = fmt.Sprintf("%v", v)
	}

	finalHeaders := make(map[string]string)
	rule := s.matchRule(url, docUrl)

	if rule != nil {
		for _, key := range rule.CaptureHeaders {
			for k, v := range allHeaders {
				if strings.EqualFold(k, key) {
					finalHeaders[key] = v
					break
				}
			}
		}
	} else {
		keys := []string{"Referer", "Cookie", "User-Agent"}
		for _, key := range keys {
			if v, ok := allHeaders[key]; ok {
				finalHeaders[key] = v
			}
		}
	}
	return finalHeaders
}

func (s *Sniffer) handleResource(url, title, docUrl, targetID string, cdpHeaders network.Headers) {
	allHeaders := make(map[string]string)
	for k, v := range cdpHeaders {
		allHeaders[k] = fmt.Sprintf("%v", v)
	}

	finalHeaders := make(map[string]string)
	rule := s.matchRule(url, docUrl)

	if rule != nil {
		log.Printf("[规则命中: %s] %s", rule.Name, url)
		for _, key := range rule.CaptureHeaders {
			for k, v := range allHeaders {
				if strings.EqualFold(k, key) {
					finalHeaders[key] = v
					break
				}
			}
		}
	} else {
		// 通用模式：默认保留核心 Header
		keys := []string{"Referer", "Cookie", "User-Agent"}
		for _, key := range keys {
			if v, ok := allHeaders[key]; ok {
				finalHeaders[key] = v
			}
		}
	}

	event := &SniffEvent{
		Url:       url,
		Title:     title,
		OriginUrl: docUrl,
		TargetID:  targetID,
		Type:      s.getURLType(url),
		Headers:   finalHeaders,
	}
	s.manager.emitEvent("video_sniffed", event)
}

func (s *Sniffer) matchRule(url, docUrl string) *SniffRule {
	for i := range s.rules {
		r := &s.rules[i]
		if r.HostKeyword != "" && !strings.Contains(url, r.HostKeyword) {
			continue
		}
		if r.MustContain != "" && !strings.Contains(url, r.MustContain) {
			continue
		}
		if r.TargetReferer != "" && !strings.Contains(docUrl, r.TargetReferer) {
			continue
		}
		if r.UrlRegex != "" {
			if m, _ := regexp.MatchString(r.UrlRegex, url); !m {
				continue
			}
		}
		return r
	}
	return nil
}

func (s *Sniffer) isGenericMediaURL(url string) bool {
	l := strings.ToLower(url)
	return strings.Contains(l, ".m3u8") || strings.Contains(l, ".mp4") || strings.Contains(l, "/hls/")
}

func (s *Sniffer) getURLType(url string) string {
	if strings.Contains(strings.ToLower(url), ".m3u8") || strings.Contains(url, "/hls/") {
		return "hls"
	}
	return "mp4"
}

func (s *Sniffer) loadRules() []SniffRule {
	path := s.env.GetRulesPath()
	if path == "" {
		return []SniffRule{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return []SniffRule{}
	}
	var rules []SniffRule
	json.Unmarshal(data, &rules)
	return rules
}

func (s *Sniffer) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}
