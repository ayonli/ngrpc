package main

import (
	"fmt"
	"os"

	"github.com/ayonli/ngrpc/cli/ngrpc/cmd"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/host"
)

func main() {
	args := os.Args

	if len(args) > 1 && args[1] == "host-server" {
		config, err := config.LoadConfig()

		if err != nil {
			fmt.Println(err)
			return
		}

		host := host.NewHost(config)
		err = host.Start(true)

		if err != nil {
			fmt.Println(err)
		}
	} else {
		cmd.Execute()
	}
}
