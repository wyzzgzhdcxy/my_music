package main

import (
	"embed"
	"log"
	"net"
	"net/http"
	"time"
	myUtil "wcj-go-common/utils"

	"context"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	startTime := time.Now()
	myUtil.InitLog(true)
	log.Printf("[启动] ========== 程序开始启动 ========== 时间: %s", startTime.Format("15:04:05.000"))

	// 单例检查：尝试连接 HTTP 服务器端口，端口被占用说明已有实例在运行
	conn, err := net.DialTimeout("tcp", "localhost:19890", 500*time.Millisecond)
	if err == nil {
		conn.Close()
		log.Printf("[启动] 检测到已有实例在运行，将老窗口置顶")
		notifyAndExit()
		return
	}

	// Create an instance of the app structure
	app := NewApp()

	log.Printf("[启动] Wails 应用创建完成 耗时: %s", time.Since(startTime))

	// Create application with options
	err = wails.Run(&options.App{
		Title:         "我的音乐盒",
		Width:         924,
		Height:        568,
		Frameless:     true,
		DisableResize: true,
		MinWidth:      300,
		MinHeight:     30,
		MaxWidth:      924,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 15, B: 30, A: 1},
		OnStartup:        app.startup,
		OnDomReady: func(ctx context.Context) {
			log.Printf("[启动] ========== 页面 DOM 就绪 ========== 耗时: %s", time.Since(startTime))
		},
		Bind: []any{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// notifyAndExit 通知已有实例并退出
func notifyAndExit() {
	// 通过 HTTP 请求发送激活信号
	resp, err := http.Get("http://localhost:19890/activate_1964e24cbef57e7ca32f80238fd3320c")
	if err != nil {
		log.Printf("[启动] 连接已有实例失败: %v", err)
		return
	}
	resp.Body.Close()
	log.Printf("[启动] 已发送激活信号给已有实例，退出")
}
