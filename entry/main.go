package main

import (
	"log"
	"os"

	"github.com/ayonli/ngrpc"
	_ "github.com/ayonli/ngrpc/services"
)

func main() {
	appName := os.Args[1]
	app, err := ngrpc.Boot(appName)

	if err != nil {
		log.Fatal(err)
	} else {
		app.WaitForExit()
	}
}
