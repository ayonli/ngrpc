package cmd

import (
	"github.com/ayonli/ngrpc/host"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list all apps (exclude non-served ones)",
	Run: func(cmd *cobra.Command, args []string) {
		host.SendCommand("list", "")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
