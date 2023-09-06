package cmd

import (
	"github.com/ayonli/ngrpc/pm"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop [app]",
	Short: "stop an app or all apps",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			pm.SendCommand("stop", args[0])
		} else {
			pm.SendCommand("stop", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
