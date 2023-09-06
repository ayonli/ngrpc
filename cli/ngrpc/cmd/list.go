package cmd

import (
	"github.com/ayonli/ngrpc/pm"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "list all apps (exclude non-served ones)",
	Run: func(cmd *cobra.Command, args []string) {
		pm.SendCommand("list", "")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
