package cmd

import (
	"fmt"

	"github.com/ayonli/ngrpc/pm"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [app]",
	Short: "start an app or all apps (exclude non-served ones)",
	Run: func(cmd *cobra.Command, args []string) {
		if !pm.IsHostOnline() {
			err := startHost(false)

			if err != nil {
				fmt.Println(err)
				return
			}
		}

		if len(args) > 0 {
			pm.SendCommand("start", args[0])
		} else {
			pm.SendCommand("start", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
