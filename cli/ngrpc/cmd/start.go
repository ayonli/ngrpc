package cmd

import (
	"fmt"

	"github.com/ayonli/ngrpc/host"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start [app]",
	Short: "start an app or all apps (exclude non-served ones)",
	Run: func(cmd *cobra.Command, args []string) {
		if !host.IsLive() {
			err := startHost(false)

			if err != nil {
				fmt.Println(err)
				return
			}
		}

		if len(args) > 0 {
			host.SendCommand("start", args[0])
		} else {
			host.SendCommand("start", "")
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
