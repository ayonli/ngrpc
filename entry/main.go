package main

import (
	"log"

	"github.com/ayonli/ngrpc"
	_ "github.com/ayonli/ngrpc/services"
)

func main() {
	app, err := ngrpc.Start(ngrpc.GetAppName())

	if err != nil {
		log.Fatal(err)
	} else {
		app.WaitForExit()
	}
}
