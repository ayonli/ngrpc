package ngrpc_test

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services"
	"github.com/ayonli/ngrpc/services/github/ayonli/ngrpc/services_proto"
	"github.com/ayonli/ngrpc/services/proto"
	"github.com/stretchr/testify/assert"
)

func Example() {
	app := goext.Ok(ngrpc.Start("user-server"))
	defer app.Stop()

	ctx := context.Background()
	userId := "ayon.li"
	userSrv := goext.Ok(ngrpc.GetServiceClient(&services.UserService{}, userId))
	user := goext.Ok(userSrv.GetUser(ctx, &services_proto.UserQuery{Id: &userId}))

	fmt.Println("Id:", user.Id)
	fmt.Println("Name:", user.Name)
	fmt.Println("Gender:", user.Gender)
	fmt.Println("Age:", user.Age)
	fmt.Println("Email:", user.Email)
	// Output:
	// Id: ayon.li
	// Name: A-yon Lee
	// Gender: MALE
	// Age: 28
	// Email: the@ayon.li
}

func TestGetServiceClient(t *testing.T) {
	app := goext.Ok(ngrpc.Start("user-server"))
	defer app.Stop()

	ins1 := goext.Ok(ngrpc.GetServiceClient(&services.UserService{}, ""))
	ins2 := goext.Ok(ngrpc.GetServiceClient(&services.UserService{}, "user-server"))
	ins3 := goext.Ok(ngrpc.GetServiceClient(&services.UserService{}, "grpcs://localhost:4001"))

	assert.NotNil(t, ins1)
	assert.NotNil(t, ins2)
	assert.NotNil(t, ins3)
	assert.Equal(t, ins1, ins2)
	assert.Equal(t, ins1, ins3)
}

func TestForSnippet(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start", "example-server").Run())
	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	ins := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))
	text := goext.Ok(ins.SayHello(ctx, &proto.HelloRequest{Name: "A-yon Lee"}))

	assert.Equal(t, "Hello, A-yon Lee", text.Message)

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Second) // Host.Stop waited a while for message flushing, we wait here too
}
