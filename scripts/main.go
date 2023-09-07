package main

import (
	"context"
	"fmt"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services"
	"github.com/ayonli/ngrpc/services/github/ayonli/ngrpc/services_proto"
	"github.com/ayonli/ngrpc/services/proto"
)

func main() {
	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	userId := "ayon.li"
	userSrv := goext.Ok((&services.UserService{}).GetClient(userId))

	user := goext.Ok(userSrv.GetUser(ctx, &services_proto.UserQuery{Id: &userId}))
	fmt.Println(user)

	posts := goext.Ok(userSrv.GetMyPosts(ctx, &services_proto.UserQuery{Id: &userId}))
	fmt.Println(posts)

	expSrv := goext.Ok((&services.ExampleService{}).GetClient(""))
	reply := goext.Ok(expSrv.SayHello(ctx, &proto.HelloRequest{Name: "World"}))
	fmt.Println(reply.Message)
}
