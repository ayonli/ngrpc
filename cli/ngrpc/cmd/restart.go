package cmd

import (
	"github.com/ayonli/ngrpc/pm"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [app]",
	Short: "restart an app or all apps",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			pm.SendCommand("restart", args[0])
		} else {
			pm.SendCommand("restart", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
