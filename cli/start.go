package cli

import (
	"time"

	"github.com/nhirsama/Naniwosuruno/client"
	"github.com/nhirsama/Naniwosuruno/server"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start both client and server",
	Run: func(cmd *cobra.Command, args []string) {
		// 启动服务端 (非阻塞)
		go server.Run()

		// 稍微等待服务端初始化
		time.Sleep(500 * time.Millisecond)

		// 启动客户端 (阻塞)
		client.Run()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
