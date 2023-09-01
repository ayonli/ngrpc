package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ayonli/goext/stringx"
	"github.com/ayonli/ngrpc/util"
	"github.com/spf13/cobra"
)

var confTpl = `{
	"$schema": "https://raw.githubusercontent.com/ayonli/ngrpc/main/ngrpc.schema.json",
    "entry": "main.go",
    "apps": [
        {
            "name": "example-server",
            "uri": "grpc://localhost:4000",
            "serve": true,
            "services": [
                "services.ExampleService"
            ],
            "stdout": "out.log"
        }
    ]
}`

var mainGoTpl = `package main

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
`

var exampleProtoTpl = `syntax = "proto3";

option go_package = "./proto";

package services;

message HelloRequest {
    string name = 1;
}

message HelloReply {
    string message = 2;
}

service ExampleService {
    rpc sayHello(HelloRequest) returns (HelloReply) {}
}
`

var exampleServiceTpl = `package services

import (
	"context"

	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services/proto"
	"google.golang.org/grpc"
)

type ExampleService struct {
	proto.UnimplementedExampleServiceServer
}

func (self *ExampleService) Serve(s grpc.ServiceRegistrar) {
	proto.RegisterExampleServiceServer(s, self)
}

func (self *ExampleService) Connect(cc grpc.ClientConnInterface) proto.ExampleServiceClient {
	return proto.NewExampleServiceClient(cc)
}

func (self *ExampleService) GetClient(route string) (proto.ExampleServiceClient, error) {
	return ngrpc.GetServiceClient(self, route)
}

func (self *ExampleService) SayHello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello, " + req.Name}, nil
}

func init() {
	ngrpc.Use(&ExampleService{})
}
`

var shTpl = `
echo "generating code according to the proto files..."
protoc --proto_path=proto --go_out=./services --go-grpc_out=./services proto/*.proto
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initiate a new ngrpc project",
	Run: func(cmd *cobra.Command, args []string) {
		confFile := "ngrpc.json"
		entryFile := "main.go"
		protoDir := "proto"
		protoFile := "proto/ExampleService.proto"
		servicesDir := "services"
		serviceFile := "services/ExampleService.go"
		shFile := "code-gen.sh"

		var modName string
		data, err := os.ReadFile("go.mod")

		if err == nil {
			match := stringx.Match(string(data), `module (\S+)`)

			if match != nil {
				modName = match[1]
			}
		}

		if util.Exists(confFile) {
			fmt.Printf("file '%s' already exists\n", confFile)
		} else {
			os.WriteFile(confFile, []byte(confTpl), 0644)
			fmt.Printf("config file written to '%s'\n", confFile)
		}

		if util.Exists(entryFile) {
			fmt.Printf("entry file '%s' already exists\n", entryFile)
		} else {
			tpl := mainGoTpl
			var hasError bool

			if modName != "" {
				tpl = strings.Replace(
					tpl,
					"github.com/ayonli/ngrpc/services",
					modName+"/services",
					1)
			} else {
				hasError = true
			}

			os.WriteFile(entryFile, []byte(tpl), 0644)
			fmt.Printf("entry file written to '%s'\n", entryFile)

			if hasError {
				fmt.Println("")
				fmt.Printf(
					"Warning: cannot determine the current go module, '%s' uses the default service path\n",
					entryFile)
				fmt.Println("")
			}
		}

		if util.Exists(protoDir) {
			fmt.Printf("path '%s' already exists\n", protoDir)
		} else {
			util.EnsureDir(protoDir)
			fmt.Printf("path '%s' created\n", protoDir)
		}

		if util.Exists(servicesDir) {
			fmt.Printf("path '%s' already exists\n", servicesDir)
		} else {
			util.EnsureDir(servicesDir)
			fmt.Printf("path '%s' created\n", servicesDir)
		}

		if util.Exists(protoFile) {
			fmt.Printf("file '%s' already exists\n", protoFile)
		} else {
			os.WriteFile(protoFile, []byte(exampleProtoTpl), 0644)
			fmt.Printf("example proto file written to '%s'\n", protoFile)
		}

		if util.Exists(serviceFile) {
			fmt.Printf("file '%s' already exists\n", serviceFile)
		} else {
			tpl := exampleServiceTpl
			var hasError bool

			if modName != "" {
				tpl = strings.Replace(
					tpl,
					"github.com/ayonli/ngrpc/services",
					modName+"/services",
					1)
			} else {
				hasError = true
			}

			os.WriteFile(serviceFile, []byte(tpl), 0644)
			fmt.Printf("example service file written to '%s'\n", serviceFile)

			if hasError {
				fmt.Println("")
				fmt.Printf(
					"Warning: cannot determine the current go module, '%v' uses the default service path\n",
					serviceFile)
				fmt.Println("")
			}
		}

		if util.Exists(shFile) {
			fmt.Printf("file '%s' already exists\n", shFile)
		} else {
			os.WriteFile(shFile, []byte(shTpl), 0644)
			fmt.Printf("shell file written to '%s'\n", shFile)

			cmd := exec.Command("chmod", "+x", shFile)
			err := cmd.Run()

			if err != nil {
				fmt.Println(err)
			} else {
				cmd = exec.Command("bash", "./"+shFile)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = cmd.Run()

				if err != nil {
					fmt.Println(err)
				} else {
					cmd = exec.Command("go", "mod", "tidy")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()

					if err != nil {
						fmt.Println(err)
					} else {
						fmt.Println("")
						fmt.Println("All procedures finished, now try the following command to start your first gRPC app")
						fmt.Println("")
						fmt.Println("    ngrpc start")
					}
				}
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
