package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/ayonli/ngrpc/host"
	"github.com/spf13/cobra"
)

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "start the host server in standalone mode",
	Run: func(cmd *cobra.Command, args []string) {
		flag := cmd.Flag("stop")

		if flag != nil && flag.Value.String() == "true" {
			if !host.IsLive() {
				fmt.Println("host server is not running")
			} else {
				host.SendCommand("stop-host", "")
			}
		} else if host.IsLive() {
			fmt.Println("host server is already running")
		} else {
			err := startHost(true)

			if err != nil {
				fmt.Println(err)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(hostCmd)
	hostCmd.Flags().Bool("stop", false, "stop the host server")
}

func startHost(standalone bool) error {
	cmd := exec.Command(os.Args[0], "host-server")

	if standalone {
		cmd.Args = append(cmd.Args, "--standalone")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()

	if err != nil {
		return err
	} else {
		cmd.Process.Release()
		time.Sleep(time.Millisecond * 200) // wait a while for the host server to serve
		return nil
	}
}
