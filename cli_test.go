//go:build !windows
// +build !windows

package ngrpc_test

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/goext/stringx"
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services"
	"github.com/ayonli/ngrpc/services/github/ayonli/ngrpc/services_proto"
	"github.com/ayonli/ngrpc/services/proto"
	"github.com/stretchr/testify/assert"
)

func TestHostCommand(t *testing.T) {
	output := goext.Ok(exec.Command("ngrpc", "host").Output())
	assert.Contains(t, string(output), "host server started")

	exam := goext.Ok(exec.Command("ps", "aux").Output())
	assert.Contains(t, string(exam), "ngrpc host-server --standalone")

	output = goext.Ok(exec.Command("ngrpc", "host", "--stop").Output())
	assert.Contains(t, string(output), "host server shut down")

	time.Sleep(time.Millisecond * 10)
	exam = goext.Ok(exec.Command("ps", "aux").Output())
	assert.NotContains(t, string(exam), "ngrpc host-server --standalone")
}

func TestStartAndStopCommand_singleApp(t *testing.T) {
	output := goext.Ok(exec.Command("ngrpc", "start", "example-server").Output())
	assert.Contains(t, string(output), "app [example-server] started")

	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	srv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))
	reply := goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hello, World", reply.Message)

	output = goext.Ok(exec.Command("ngrpc", "stop", "example-server").Output())
	assert.Contains(t, string(output), "app [example-server] stopped")

	exam := goext.Ok(exec.Command("ps", "aux").Output())
	assert.NotContains(t, string(exam), "example-server")

	goext.Ok(0, exec.Command("ngrpc", "host", "--stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestStartAndStopCommand_allApps(t *testing.T) {
	output := goext.Ok(exec.Command("ngrpc", "start").Output())
	assert.Contains(t, string(output), "host server started")
	assert.Contains(t, string(output), "app [example-server] started")
	assert.Contains(t, string(output), "app [user-server] started")
	assert.Contains(t, string(output), "app [post-server] started")

	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	exampleSrv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))
	reply := goext.Ok((exampleSrv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hello, World", reply.Message)

	userId := "ayon.li"
	userSrv := goext.Ok(ngrpc.GetServiceClient(&services.UserService{}, userId))
	user := goext.Ok(userSrv.GetUser(ctx, &services_proto.UserQuery{Id: &userId}))
	assert.Equal(t, "A-yon Lee", user.Name)

	postSrv := goext.Ok(ngrpc.GetServiceClient(&services.PostService{}, userId))
	posts := goext.Ok(postSrv.SearchPosts(ctx, &services_proto.PostsQuery{Author: &userId}))
	assert.True(t, len(posts.Posts) > 0)

	output = goext.Ok(exec.Command("ngrpc", "stop").Output())
	assert.Contains(t, string(output), "app [example-server] stopped")
	assert.Contains(t, string(output), "app [user-server] stopped")
	assert.Contains(t, string(output), "app [post-server] stopped")
	assert.Contains(t, string(output), "host server shut down")

	time.Sleep(time.Millisecond * 10) // for system to release resources
	exam := goext.Ok(exec.Command("ps", "aux").Output())
	assert.NotContains(t, string(exam), "example-server")
	assert.NotContains(t, string(exam), "user-server")
	assert.NotContains(t, string(exam), "post-server")
}

func TestListCommand(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start").Run())
	output := goext.Ok(exec.Command("ngrpc", "list").Output())
	rows := strings.Split(string(output), "\n")

	for i, row := range rows {
		columns := strings.Fields(row)

		if i == 0 {
			assert.Equal(t, "App", columns[0])
			assert.Equal(t, "URL", columns[1])
			assert.Equal(t, "Status", columns[2])
			assert.Equal(t, "Pid", columns[3])
			assert.Equal(t, "Uptime", columns[4])
			assert.Equal(t, "Memory", columns[5])
			assert.Equal(t, "CPU", columns[6])
		} else if i == 1 {
			assert.Equal(t, "example-server", columns[0])
			assert.Equal(t, "grpc://localhost:4000", columns[1])
			assert.Equal(t, "running", columns[2])
			assert.NotNil(t, stringx.Match(columns[3], "^\\d+$"))
			assert.NotNil(t, stringx.Match(columns[4], "^\\ds$"))
		} else if i == 2 {
			assert.Equal(t, "user-server", columns[0])
			assert.Equal(t, "grpcs://localhost:4001", columns[1])
			assert.Equal(t, "running", columns[2])
			assert.NotNil(t, stringx.Match(columns[3], "^\\d+$"))
			assert.NotNil(t, stringx.Match(columns[4], "^\\ds$"))
		} else if i == 3 {
			assert.Equal(t, "post-server", columns[0])
			assert.Equal(t, "grpcs://localhost:4002", columns[1])
			assert.Equal(t, "running", columns[2])
			assert.NotNil(t, stringx.Match(columns[3], "^\\d+$"))
			assert.NotNil(t, stringx.Match(columns[4], "^\\ds$"))
		}
	}

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)

	output = goext.Ok(exec.Command("ngrpc", "list").Output())
	rows = strings.Split(string(output), "\n")

	for i, row := range rows {
		columns := strings.Fields(row)

		if i == 0 {
			assert.Equal(t, "App", columns[0])
			assert.Equal(t, "URL", columns[1])
			assert.Equal(t, "Status", columns[2])
			assert.Equal(t, "Pid", columns[3])
			assert.Equal(t, "Uptime", columns[4])
			assert.Equal(t, "Memory", columns[5])
			assert.Equal(t, "CPU", columns[6])
		} else if i == 1 {
			assert.Equal(t, "example-server", columns[0])
			assert.Equal(t, "grpc://localhost:4000", columns[1])
			assert.Equal(t, "stopped", columns[2])
			assert.Equal(t, "N/A", columns[3])
			assert.Equal(t, "N/A", columns[4])
			assert.Equal(t, "N/A", columns[5])
			assert.Equal(t, "N/A", columns[6])
		} else if i == 2 {
			assert.Equal(t, "user-server", columns[0])
			assert.Equal(t, "grpcs://localhost:4001", columns[1])
			assert.Equal(t, "stopped", columns[2])
			assert.Equal(t, "N/A", columns[3])
			assert.Equal(t, "N/A", columns[4])
			assert.Equal(t, "N/A", columns[5])
			assert.Equal(t, "N/A", columns[6])
		} else if i == 3 {
			assert.Equal(t, "post-server", columns[0])
			assert.Equal(t, "grpcs://localhost:4002", columns[1])
			assert.Equal(t, "stopped", columns[2])
			assert.Equal(t, "N/A", columns[3])
			assert.Equal(t, "N/A", columns[4])
			assert.Equal(t, "N/A", columns[5])
			assert.Equal(t, "N/A", columns[6])
		}
	}
}

func TestReloadCommand_singleApp(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start", "example-server").Run())

	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	srv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))
	reply := goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hello, World", reply.Message)

	oldContents := string(goext.Ok(os.ReadFile("services/ExampleService.ts")))
	newContents := strings.Replace(oldContents, `"Hello, "`, `"Hi, "`, 1)
	goext.Ok(0, os.WriteFile("services/ExampleService.ts", []byte(newContents), 0644))
	defer os.WriteFile("services/ExampleService.ts", []byte(oldContents), 0644)

	output := goext.Ok(exec.Command("ngrpc", "reload", "example-server").Output())
	assert.Contains(t, string(output), "app [example-server] hot-reloaded")

	reply = goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hi, World", reply.Message)

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestReloadCommand_allApps(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start").Run())

	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	srv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))
	reply := goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hello, World", reply.Message)

	oldContents := string(goext.Ok(os.ReadFile("services/ExampleService.ts")))
	newContents := strings.Replace(oldContents, `"Hello, "`, `"Hi, "`, 1)
	goext.Ok(0, os.WriteFile("services/ExampleService.ts", []byte(newContents), 0644))
	defer os.WriteFile("services/ExampleService.ts", []byte(oldContents), 0644)

	output := goext.Ok(exec.Command("ngrpc", "reload").Output())
	assert.Contains(t, string(output), "app [example-server] hot-reloaded")
	assert.Contains(t, string(output), "app [post-server] hot-reloaded")
	assert.Contains(t, string(output), "app [user-server] does not support hot-reloading")

	reply = goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hi, World", reply.Message)

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestRestartCommand_singleApp(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start", "example-server").Run())

	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	srv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))
	reply := goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hello, World", reply.Message)

	oldContents := string(goext.Ok(os.ReadFile("services/ExampleService.ts")))
	newContents := strings.Replace(oldContents, `"Hello, "`, `"Hi, "`, 1)
	goext.Ok(0, os.WriteFile("services/ExampleService.ts", []byte(newContents), 0644))
	defer os.WriteFile("services/ExampleService.ts", []byte(oldContents), 0644)

	output := goext.Ok(exec.Command("ngrpc", "restart", "example-server").Output())
	assert.Contains(t, string(output), "app [example-server] stopped")
	assert.Contains(t, string(output), "app [example-server] started")

	reply = goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hi, World", reply.Message)

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestRestartCommand_allApps(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start").Run())

	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	srv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))
	reply := goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hello, World", reply.Message)

	oldContents := string(goext.Ok(os.ReadFile("services/ExampleService.ts")))
	newContents := strings.Replace(oldContents, `"Hello, "`, `"Hi, "`, 1)
	goext.Ok(0, os.WriteFile("services/ExampleService.ts", []byte(newContents), 0644))
	defer os.WriteFile("services/ExampleService.ts", []byte(oldContents), 0644)

	output := goext.Ok(exec.Command("ngrpc", "restart").Output())
	assert.Contains(t, string(output), "app [example-server] stopped")
	assert.Contains(t, string(output), "app [example-server] started")
	assert.Contains(t, string(output), "app [user-server] stopped")
	assert.Contains(t, string(output), "app [user-server] started")
	assert.Contains(t, string(output), "app [post-server] stopped")
	assert.Contains(t, string(output), "app [post-server] started")

	reply = goext.Ok((srv.SayHello(ctx, &proto.HelloRequest{Name: "World"})))
	assert.Equal(t, "Hi, World", reply.Message)

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestRunCommand_go(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start").Run())

	output := goext.Ok(exec.Command("ngrpc", "run", "scripts/main.go").Output())
	assert.Contains(t, string(output), "Hello, World")

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}

func TestRunCommand_ts(t *testing.T) {
	goext.Ok(0, exec.Command("ngrpc", "start").Run())

	output := goext.Ok(exec.Command("ngrpc", "run", "scripts/main.ts").Output())
	assert.Contains(t, string(output), "Hello, World")

	goext.Ok(0, exec.Command("ngrpc", "stop").Run())
	time.Sleep(time.Millisecond * 10)
}
