package engine

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// ProxyServer 处理本地预览的代理请求
type ProxyServer struct {
	Port int
}

func NewProxyServer(port int) *ProxyServer {
	return &ProxyServer{Port: port}
}

// Start 启动本地代理服务
func (s *ProxyServer) Start() {
	http.HandleFunc("/proxy", s.handleProxy)
	// 在协程中启动，不阻塞主进程
	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", s.Port)
		fmt.Printf("本地代理服务器启动: http://%s\n", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			fmt.Printf("代理服务器启动失败: %v\n", err)
		}
	}()
}

// handleProxy 处理具体的请求转发逻辑
func (s *ProxyServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	// 获取前端传来的目标 URL 和可能的伪造 Referer
	targetURL := r.URL.Query().Get("url")
	referer := r.URL.Query().Get("referer")

	if targetURL == "" {
		http.Error(w, "缺少 url 参数", http.StatusBadRequest)
		return
	}

	// 1. 创建转发请求
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. 伪造关键请求头，绕过防盗链
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if referer != "" {
		req.Header.Set("Referer", referer)
	} else {
		// 如果没传，默认尝试使用目标域名的根路径作为 Referer
		if u, err := url.Parse(targetURL); err == nil {
			req.Header.Set("Referer", fmt.Sprintf("%s://%s/", u.Scheme, u.Host))
		}
	}

	// 3. 执行请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// 4. 关键：添加允许跨域的响应头，让 Wails 里的 React 能正常读取流
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))

	// 处理视频流特有的 Range 请求支持（如果视频源支持，我们也转发）
	if resp.Header.Get("Accept-Ranges") != "" {
		w.Header().Set("Accept-Ranges", resp.Header.Get("Accept-Ranges"))
	}
	if resp.Header.Get("Content-Range") != "" {
		w.Header().Set("Content-Range", resp.Header.Get("Content-Range"))
		w.WriteHeader(http.StatusPartialContent)
	}

	// 5. 将视频数据流式传输给前端
	io.Copy(w, resp.Body)
}
