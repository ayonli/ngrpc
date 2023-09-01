package cmd

import (
	"github.com/ayonli/ngrpc/host"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [app]",
	Short: "stop an app or all apps",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			host.SendCommand("stop", args[0])
		} else {
			host.SendCommand("stop", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
