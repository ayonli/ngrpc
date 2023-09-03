package ngrpc

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/ayonli/goext"
	"github.com/ayonli/goext/structx"
	"github.com/ayonli/ngrpc/config"
	"github.com/ayonli/ngrpc/host"
	"github.com/ayonli/ngrpc/services/github/ayonli/services_proto"
	"github.com/ayonli/ngrpc/util"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

type UserService struct{}

func (self *UserService) Serve(s grpc.ServiceRegistrar) {
	services_proto.RegisterUserServiceServer(s, nil)
}

func (self *UserService) Connect(cc grpc.ClientConnInterface) services_proto.UserServiceClient {
	return services_proto.NewUserServiceClient(cc)
}

type PostService struct {
	User *UserService // Used to test DI
}

func (self *PostService) Serve(s grpc.ServiceRegistrar) {
	services_proto.RegisterPostServiceServer(s, nil)
}

func (self *PostService) Connect(cc grpc.ClientConnInterface) services_proto.PostServiceClient {
	return services_proto.NewPostServiceClient(cc)
}

type UnservableService struct{}

func (self *UnservableService) Connect(cc grpc.ClientConnInterface) services_proto.PostServiceClient {
	return services_proto.NewPostServiceClient(cc)
}

type UnregisteredService struct{}

func (self *UnregisteredService) Connect(cc grpc.ClientConnInterface) services_proto.PostServiceClient {
	return services_proto.NewPostServiceClient(cc)
}

func init() {
	Use(&UserService{})
	Use(&PostService{})
	Use(&UnservableService{})
}

func TestInitServer(t *testing.T) {
	var conf = config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpc://localhost:5001",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
			{
				Name: "server-2",
				Uri:  "grpc://localhost:5002",
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initServer()

	defer app.Stop()

	assert.Nil(t, err)
	assert.NotNil(t, app.server)

	// test dependency injection
	userService := app.services[0].(*UserService)
	postService := app.services[1].(*PostService)
	assert.Equal(t, postService.User, userService)
}

func TestInitServerXdsUri(t *testing.T) {
	var conf = config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "xds://localhost:5001",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initServer()

	assert.Equal(t, "app [server-1] cannot be served since it uses 'xds:' protocol", err.Error())
}

func TestInitServerInvalidUri(t *testing.T) {
	var conf = config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpc://localhost:abc",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initServer()

	assert.Equal(t,
		"parse \"grpc://localhost:abc\": invalid port \":abc\" after host",
		err.Error())
}

func TestInitServerInvalidCredentials(t *testing.T) {
	var conf = config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpcs://localhost:6000",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
				},
				Ca:   "./certs/ca.pem",
				Cert: "./certs/cert.pem",
				Key:  "../certs/cert.key",
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initServer()

	assert.Equal(t, "open ../certs/cert.key: no such file or directory", err.Error())
}

func TestInitServerUnregisteredService(t *testing.T) {
	var conf = config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "grpc://localhost:6000",
				Services: []string{
					getServiceName(&UnregisteredService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initServer()

	assert.Equal(t, "service [ngrpc.UnregisteredService] hasn't been registered", err.Error())
}

func TestInitServerUnservableService(t *testing.T) {
	var conf = config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "grpc://localhost:6000",
				Services: []string{
					getServiceName(&UnservableService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initServer()

	assert.Equal(t, "service [ngrpc.UnservableService] doesn't implement the Serve() method", err.Error())
}

func TestInitServerUnavailablePort(t *testing.T) {
	app1 := &RpcApp{
		App: config.App{
			Name:  "server-1",
			Uri:   "grpc://localhost:6000",
			Serve: true,
			Services: []string{
				getServiceName(&UserService{}),
			},
		},
	}
	app2 := &RpcApp{
		App: config.App{
			Name:  "server-2",
			Uri:   "grpc://localhost:6000",
			Serve: true,
			Services: []string{
				getServiceName(&UserService{}),
			},
		},
	}

	err1 := app1.initServer()
	err2 := app2.initServer()

	defer app1.Stop()

	assert.Nil(t, err1)
	assert.Contains(t, err2.Error(), "bind: address already in use")
}

func TestInitClient(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "grpc://localhost:6001",
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
			{
				Name: "server-2",
				Uri:  "grpc://localhost:6002",
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initClient(conf.Apps)

	assert.Nil(t, err)
	assert.Equal(t, 2, app.serviceDialers.Size())
}

func TestInitClientTLS(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "grpcs://localhost:6000",
				Services: []string{
					getServiceName(&UserService{}),
				},
				Ca:   "./certs/ca.pem",
				Cert: "./certs/cert.pem",
				Key:  "./certs/cert.key",
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initClient(conf.Apps)

	assert.Nil(t, err)
	assert.Equal(t, 1, app.serviceDialers.Size())
}

func TestInitClientXdsUri(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "xds://localhost:6000",
				Services: []string{
					getServiceName(&UserService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initClient(conf.Apps)

	assert.Nil(t, err)
	assert.Equal(t, 1, app.serviceDialers.Size())
}

func TestInitClientInvalidUri(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "grpc://localhost:abc",
				Services: []string{
					getServiceName(&UserService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initClient(conf.Apps)

	assert.Equal(t,
		"parse \"grpc://localhost:abc\": invalid port \":abc\" after host",
		err.Error())
}

func TestInitClientInvalidCredentials(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "grpcs://localhost:6000",
				Services: []string{
					getServiceName(&UserService{}),
				},
				Ca:   "./certs/ca.pem",
				Cert: "./certs/cert.pem",
				Key:  "../certs/cert.key",
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initClient(conf.Apps)

	assert.Equal(t, "open ../certs/cert.key: no such file or directory", err.Error())
}

func TestInitClientUnregisteredService(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name: "server-1",
				Uri:  "grpc://localhost:6000",
				Services: []string{
					getServiceName(&UnregisteredService{}),
				},
			},
		},
	}
	app := &RpcApp{App: conf.Apps[0]}
	err := app.initClient(conf.Apps)

	assert.Equal(t, "service [ngrpc.UnregisteredService] hasn't been registered", err.Error())
}

func TestStartDuplicateCall(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpc://localhost:5003",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
			{
				Name: "server-2",
				Uri:  "grpc://localhost:5004",
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}

	app1, err1 := StartWithConfig("server-1", conf)
	app2, err2 := StartWithConfig("server-2", conf)

	defer app1.Stop()

	assert.NotNil(t, app1)
	assert.Nil(t, err1)
	assert.Nil(t, app2)
	assert.Equal(t, "an app is already running", err2.Error())
}

func TestStartInvalidApp(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpc://localhost:5003",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
			{
				Name: "server-2",
				Uri:  "grpc://localhost:5004",
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}

	app, err := StartWithConfig("server-3", conf)

	assert.Nil(t, app)
	assert.Equal(t, "app [server-3] is not configured", err.Error())
}

func TestStartCantInitServer(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpc://localhost:abc",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}

	app, err := StartWithConfig("server-1", conf)

	assert.Nil(t, app)
	assert.Equal(t, "parse \"grpc://localhost:abc\": invalid port \":abc\" after host", err.Error())
}

func TestStartPureClientApp(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpc://localhost:6000",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}

	app, err := StartWithConfig("", conf)

	assert.Nil(t, err)
	assert.Equal(t, 2, app.serviceDialers.Size())

	app.Stop()
}

func TestWaitForExit(t *testing.T) {
	conf := config.Config{
		Apps: []config.App{
			{
				Name:  "server-1",
				Uri:   "grpc://localhost:5005",
				Serve: true,
				Services: []string{
					getServiceName(&UserService{}),
					getServiceName(&PostService{}),
				},
			},
		},
	}

	app, _ := StartWithConfig("server-1", conf)

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

func TestSpawnApp(t *testing.T) {
	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := cfg.Apps[0]
	pid := goext.Ok(host.SpawnApp(app, tsCfg))

	assert.NotEqual(t, -1, pid)
	assert.NotEqual(t, 0, pid)

	time.Sleep(time.Second)
	host.SendCommand("stop", "")
}

func TestSpawnApp_builtEntry(t *testing.T) {
	cmd := exec.Command("go", "build", "-o", "entry/main", "entry/main.go")
	cmd.Run()

	assert.True(t, util.Exists("entry/main"))

	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := cfg.Apps[0]
	app.Entry = "entry/main"
	pid := goext.Ok(host.SpawnApp(app, tsCfg))

	assert.NotEqual(t, -1, pid)
	assert.NotEqual(t, 0, pid)

	time.Sleep(time.Second)
	host.SendCommand("stop", "")

	os.Remove("entry/main")
}

func TestSpawnApp_pipeStdout(t *testing.T) {
	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := cfg.Apps[0]
	app.Stdout = ""
	app.Stderr = ""
	pid := goext.Ok(host.SpawnApp(app, tsCfg))

	assert.NotEqual(t, -1, pid)
	assert.NotEqual(t, 0, pid)

	time.Sleep(time.Second)
	host.SendCommand("stop", "")
}

func TestSpawnApp_invalidEntry(t *testing.T) {
	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := cfg.Apps[0]
	app.Entry = ""

	pid, err := host.SpawnApp(app, tsCfg)

	assert.Equal(t, 0, pid)
	assert.Equal(t, "entry file is not set", err.Error())
}

func TestSpawnApp_otherOptions(t *testing.T) {
	cfg := goext.Ok(config.LoadConfig())
	tsCfg := goext.Ok(config.LoadTsConfig(cfg.Tsconfig))
	app := structx.Merge(cfg.Apps[0], config.App{
		Stderr: "err.log",
		Env: map[string]string{
			"FOO": "BAR",
		},
	})
	pid := goext.Ok(host.SpawnApp(app, tsCfg))

	assert.NotEqual(t, -1, pid)
	assert.NotEqual(t, 0, pid)

	time.Sleep(time.Second)
	host.SendCommand("stop", "")
}
