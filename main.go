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
		Title:         "FetchReel",
		Width:         380, // 匹配 Edge 下载面板宽度
		Height:        720, // 初始高度
		MinWidth:      380,
		MinHeight:     400,
		Frameless:     true, // 开启无边框模式
		DisableResize: true, // 禁用手动拉伸，由程序控制宽度
		AlwaysOnTop:   true, // 默认置顶
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 26, G: 27, B: 30, A: 1}, // 匹配 Mantine 颜色
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
		// Windows 平台特定优化
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
			BackdropType:                      windows.Auto, // 尝试开启系统级云母/压克力效果
			DisableFramelessWindowDecorations: false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
