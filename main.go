package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 1. 创建 App 实例
	app := NewApp()

	// 2. 配置 Wails 选项
	err := wails.Run(&options.App{
		Title:         "FetchReel Sidebar",
		Width:         350,   // 初始侧边栏宽度
		Height:        768,   // 初始高度（建议后续在 app.go 中根据屏幕高度动态调整）
		DisableResize: false, // 允许代码调整大小（用于裁切模式扩宽）
		AlwaysOnTop:   true,  // 侧边栏强制置顶
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 27, B: 30, A: 1}, // 匹配 Mantine Dark 主题色
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		// Windows 平台特定优化
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: true, // 启动时直接弹出调试窗口，省去按 F12
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
