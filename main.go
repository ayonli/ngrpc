package main

import (
	"log"

	"github.com/ayonli/gorpc"
	"github.com/hyurl/grpc-boot/services"
)

func main() {
	gorpc.Use(&services.UserService{})
	gorpc.Use(&services.PostService{})

	app, err := gorpc.Boot("user-server")

	if err != nil {
		log.Fatal(err)
	} else {
		defer app.WaitForExit()
	}

	// postSrv := gorpc.GetServiceClient(&services.PostService{}, "")
	// ctx := context.Background()
	// post, err := postSrv.GetPost(ctx, &ayonli.PostQuery{Id: 1})

	// if err != nil {
	// 	log.Fatal(err)
	// } else {
	// 	fmt.Println(post)
	// }
}
