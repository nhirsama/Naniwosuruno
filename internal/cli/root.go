package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "naniwosuruno",
	Short: "A tool to track your focused window",
}

// Execute Run 执行根命令
func Execute() {
	// 简单的 i18n 处理
	// 检查环境变量 LANG 是否包含 zh (例如 zh_CN.UTF-8)
	lang := os.Getenv("LANG")
	if strings.Contains(lang, "zh") {
		rootCmd.Short = "一个用于追踪当前焦点窗口的工具"

		// 动态更新子命令的描述
		if clientCmd != nil {
			clientCmd.Short = "启动客户端"
		}
		if serverCmd != nil {
			serverCmd.Short = "启动服务端"
		}
		if startCmd != nil {
			startCmd.Short = "同时启动客户端和服务端"
		}
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
