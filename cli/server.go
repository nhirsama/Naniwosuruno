package cli

import (
	"github.com/nhirsama/Naniwosuruno/server"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the server",
	Run: func(cmd *cobra.Command, args []string) {
		server.Run()
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
}
