package ngrpc_test

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/services"
	"github.com/ayonli/ngrpc/services/github/ayonli/ngrpc/services_proto"
	"github.com/ayonli/ngrpc/services/proto"
	"github.com/stretchr/testify/assert"
)

func TestStart(t *testing.T) {
	app := goext.Ok(ngrpc.Start("user-server"))
	defer app.Stop()

	assert.Equal(t, "user-server", app.Name)

	userId := "ayon.li"
	userSrv := goext.Ok((&services.UserService{}).GetClient(""))
	user := goext.Ok(userSrv.GetUser(context.Background(), &services_proto.UserQuery{Id: &userId}))

	assert.Equal(t, "A-yon Lee", user.Name)
}

func TestStartWithoutAppName(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start", "example-server").Run())

	app := goext.Ok(ngrpc.Start(""))
	defer app.Stop()

	srv := goext.Ok((&services.ExampleService{}).GetClient(""))
	reply := goext.Ok(srv.SayHello(context.Background(), &proto.HelloRequest{Name: "World"}))
	assert.Equal(t, "Hello, World", reply.Message)

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestStartWithConfig(t *testing.T) {
	cfg := goext.Ok(config.LoadConfig())
	app := goext.Ok(ngrpc.StartWithConfig("user-server", cfg))
	defer app.Stop()

	assert.Equal(t, "user-server", app.Name)

	userId := "ayon.li"
	userSrv := goext.Ok((&services.UserService{}).GetClient(""))
	user := goext.Ok(userSrv.GetUser(context.Background(), &services_proto.UserQuery{Id: &userId}))

	assert.Equal(t, "A-yon Lee", user.Name)
}

func TestStartWithConfigWithXdsProtocol(t *testing.T) {
	cfg := config.Config{
		Apps: []config.App{
			{
				Name:  "example-server",
				Uri:   "xds://localhost:5001",
				Serve: true,
				Services: []string{
					"services.ExampleService",
				},
			},
		},
	}
	app, err := ngrpc.StartWithConfig("example-server", cfg)

	assert.Nil(t, app)
	assert.Equal(t,
		"app [example-server] cannot be served since it uses 'xds:' protocol",
		err.Error())
}

func TestStartInvalidApp(t *testing.T) {
	app, err := ngrpc.Start("test-server")

	assert.Nil(t, app)
	assert.Equal(t, "app [test-server] is not configured", err.Error())
}

func TestStartInvalidUri(t *testing.T) {
	cfg := config.Config{
		Apps: []config.App{
			{
				Name:  "example-server",
				Uri:   "grpc://localhost:abc",
				Serve: true,
				Services: []string{
					"services.ExampleService",
				},
			},
		},
	}

	app, err := ngrpc.StartWithConfig("example-server", cfg)

	assert.Nil(t, app)
	assert.Equal(t, "parse \"grpc://localhost:abc\": invalid port \":abc\" after host", err.Error())
}

func TestStartDuplicateCall(t *testing.T) {
	app1 := goext.Ok(ngrpc.Start("user-server"))
	app2, err := ngrpc.Start("user-server")
	defer app1.Stop()

	assert.Nil(t, app2)
	assert.Equal(t, "an app is already running", err.Error())
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
	ins := goext.Ok((&services.ExampleService{}).GetClient(""))
	text := goext.Ok(ins.SayHello(ctx, &proto.HelloRequest{Name: "A-yon Lee"}))

	assert.Equal(t, "Hello, A-yon Lee", text.Message)

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestStopAndOnStop(t *testing.T) {
	app := goext.Ok(ngrpc.Start("user-server"))
	stopped := false

	app.OnStop(func() {
		stopped = true
	})

	app.Stop()
	assert.True(t, stopped)
}

func TestWaitForExit(t *testing.T) {
	app, _ := ngrpc.Start("user-server")

	go func() {
		syscall.Kill(syscall.Getpid(), syscall.SIGINT)
	}()

	defer func() {
		if re := recover(); re != nil {
			assert.Equal(t, "unexpected call to os.Exit(0) during test", fmt.Sprint(re))
		}
	}()

	app.WaitForExit()
}
