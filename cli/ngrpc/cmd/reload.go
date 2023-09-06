package cmd

import (
	"github.com/ayonli/ngrpc/pm"
	"github.com/spf13/cobra"
)

var reloadCmd = &cobra.Command{
	Use:   "reload [app]",
	Short: "hot-reload an app or all apps",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			pm.SendCommand("reload", args[0])
		} else {
			pm.SendCommand("reload", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}
