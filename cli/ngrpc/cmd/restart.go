package cmd

import (
	"github.com/ayonli/ngrpc/host"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [app]",
	Short: "restart an app or all apps",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			host.SendCommand("restart", args[0])
		} else {
			host.SendCommand("restart", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
