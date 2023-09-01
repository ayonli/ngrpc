package cmd

import (
	"github.com/ayonli/ngrpc/host"
	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload [app]",
	Short: "reload an app or all apps",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			host.SendCommand("reload", args[0])
		} else {
			host.SendCommand("reload", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}
