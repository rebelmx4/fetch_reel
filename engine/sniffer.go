package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

func (s *Sniffer) StartBrowser() error {
	// 1. 获取当前程序运行目录
	exePath, _ := os.Executable()
	baseDir := filepath.Dir(exePath)

	// 2. 构造数据和缓存目录路径
	userDataDir := filepath.Join(baseDir, "edge_data", "user_data")
	cacheDir := filepath.Join(baseDir, "edge_data", "cache")

	// 确保目录存在
	os.MkdirAll(userDataDir, 0755)
	os.MkdirAll(cacheDir, 0755)

	// 3. 自动定位 Edge 路径 (从系统盘 Program Files (x86) 查找)
	// 通常环境变量 ProgramFiles(x86) 会指向 "C:\Program Files (x86)"
	programFiles := os.Getenv("ProgramFiles(x86)")
	if programFiles == "" {
		programFiles = `C:\Program Files (x86)` // 备选兜底
	}
	edgePath := filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe")

	// 校验文件是否存在
	if _, err := os.Stat(edgePath); os.IsNotExist(err) {
		return fmt.Errorf("找不到 Edge 浏览器: %s", edgePath)
	}

	// 4. 构造启动参数
	port := 9230
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
		fmt.Sprintf("--disk-cache-dir=%s", cacheDir),
		"--no-first-run",                   // 跳过首次运行向导
		"--no-default-browser-check",       // 不检查是否为默认浏览器
		"--remote-allow-origins=*",         // 允许所有跨域调试请求
		"--disable-infobars",               // 隐藏“正在受自动化软件控制”的提示
		"--disable-breakpad",               // 禁用崩溃汇报
		"--disable-session-crashed-bubble", // 禁用“浏览器异常关闭”的恢复提示框
		"--disable-features=msEdgeUnderstandYourData,msEdgeSidebar,msHubApps", // 禁用侧边栏和个性化数据
		"--disable-blink-features=AutomationControlled",
		"--password-store=basic",
		"--allow-insecure-localhost",
	}

	log.Printf("正在启动 Edge: %s", edgePath)
	cmd := exec.Command(edgePath, args...)

	// 5. 启动 Edge 进程
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("启动 Edge 进程失败: %v", err)
	}

	// 6. 等待浏览器完全启动
	time.Sleep(2 * time.Second)

	// 7. 启动 CDP 监听 (注意端口改为 9230)
	go s.listenToCDP(port)

	return nil
}

func (s *Sniffer) listenToCDP(port int) {
	// 1. 等待 Edge 调试端口完全就绪
	// 这一步非常重要，防止 i/o timeout
	var targetID string
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	log.Printf("等待 Edge 调试接口就绪...")
	for i := 0; i < 20; i++ { // 最多等待 10 秒
		resp, err := http.Get(fmt.Sprintf("http://%s/json/list", addr))
		if err == nil {
			var targets []map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&targets); err == nil {
				for _, t := range targets {
					// 寻找 Edge 启动时自带的那个 "新标签页" (type=page)
					if t["type"] == "page" && t["id"] != "" {
						targetID = t["id"].(string)
						break
					}
				}
			}
			resp.Body.Close()
		}
		if targetID != "" {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	if targetID == "" {
		log.Printf("错误: 无法获取 Edge 初始页面 ID")
		return
	}

	// 2. 创建分配器
	wsAddr := fmt.Sprintf("ws://%s/", addr)
	allocatorCtx, _ := chromedp.NewRemoteAllocator(context.Background(), wsAddr)

	// 3. 【核心技巧】：直接用 WithTargetID 创建第一个上下文
	// 这样 chromedp 就会直接接管现有的页面，而不会去创建一个 about:blank
	rootCtx, _ := chromedp.NewContext(allocatorCtx, chromedp.WithTargetID(target.ID(targetID)))
	// 注意：为了防止程序退出，这里暂时不调用 rootCancel()

	// 4. 立即初始化 network 等功能 (在主线程执行，确保连接稳固)
	if err := chromedp.Run(rootCtx, network.Enable()); err != nil {
		log.Printf("无法初始化 CDP 连接: %v", err)
		return
	}

	log.Printf("CDP 连接已建立，绑定到页面: %s", targetID)

	// 5. 开启全局监听，处理以后新开的标签页
	chromedp.ListenTarget(rootCtx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *target.EventTargetCreated:
			if ev.TargetInfo.Type == "page" {
				// 再次过滤 about:blank
				if ev.TargetInfo.URL == "about:blank" {
					return
				}
				tID := ev.TargetInfo.TargetID
				log.Printf("检测到新标签页: %s", tID)

				// 为新标签页创建独立的上下文并启动嗅探
				go func(id target.ID) {
					// 给浏览器一点点反应时间
					time.Sleep(200 * time.Millisecond)
					childCtx, _ := chromedp.NewContext(rootCtx, chromedp.WithTargetID(id))
					s.attachSnifferToContext(childCtx, string(id))
				}(tID)
			}
		case *target.EventTargetInfoChanged:
			if ev.TargetInfo.Type == "page" && ev.TargetInfo.Attached {
				s.manager.emitEvent("tab_focused", ev.TargetInfo.TargetID)
			}
		case *target.EventTargetDestroyed:
			s.manager.emitEvent("tab_closed", ev.TargetID)
		}
	})

	// 6. 为第一个页面（即现在的 rootCtx）启动嗅探逻辑
	// 因为 rootCtx 已经 WithTargetID 了，直接传入即可
	go s.attachSnifferToContext(rootCtx, targetID)

	// 阻塞运行
	<-rootCtx.Done()
}

// 在 sniffer.txt 中修改或添加逻辑

func (s *Sniffer) attachSnifferToContext(ctx context.Context, targetID string) {
	if ctx == nil {
		return
	}

	err := chromedp.Run(ctx, network.Enable())
	if err != nil {
		log.Printf("[Target %s] 启用 Network 失败: %v", targetID, err)
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
