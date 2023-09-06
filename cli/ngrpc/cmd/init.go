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

var tsConfTpl = `{
    "compilerOptions": {
        "module": "commonjs",
        "target": "es2018",
        "newLine": "LF",
        "incremental": true,
        "importHelpers": true,
        "sourceMap": true,
        "strict": true,
        "noUnusedParameters": true,
        "noUnusedLocals": true,
        "noImplicitAny": true,
        "noImplicitThis": true,
        "noImplicitOverride": true,
        "noImplicitReturns": true,
        "noFallthroughCasesInSwitch": true,
        "noPropertyAccessFromIndexSignature": true,
        "noUncheckedIndexedAccess": true
    },
    "include": [
        "*.ts",
        "*/**.ts"
    ]
}
`

var confTpl = `{
    "$schema": "https://raw.githubusercontent.com/ayonli/ngrpc/main/ngrpc.schema.json",
    "protoPaths": [
        "proto"
    ],
    "protoOptions": {
        "defaults": true
    },
    "apps": [
        {
            "name": "example-server",
            "url": "grpc://localhost:4000",
            "serve": true,
            "services": [
                "services.ExampleService"
            ],
            "entry": "main.go",
            "stdout": "out.log"
        }
    ]
}`

var entryGoTpl = `package main

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
`

var entryTsTpl = `import ngrpc from "@ayonli/ngrpc";

if (require.main?.filename === __filename) {
    ngrpc.start(ngrpc.getAppName()).then(app => {
        process.send?.("ready"); // for PM2
        app.waitForExit();
    }).catch(err => {
        console.error(err);
        process.exit(1);
    });
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

var exampleServiceGoTpl = `package services

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

var exampleServiceTsTpl = `import { ServiceClient, service } from "@ayonli/ngrpc";

declare global {
    namespace services {
        const ExampleService: ServiceClient<ExampleService>;
    }
}

export type HelloRequest = {
    name: string;
};

export type HelloReply = {
    message: string;
};

@service("services.ExampleService")
export default class ExampleService {
    async sayHello(req: HelloRequest): Promise<HelloReply> {
        return await Promise.resolve({ message: "Hello, " + req.name });
    }
}
`

var scriptGoTpl = `package main

import (
	"context"
	"fmt"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services"
	"github.com/ayonli/ngrpc/services/proto"
)

func main() {
	done := ngrpc.ForSnippet()
	defer done()

	ctx := context.Background()
	exampleSrv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))

	result := goext.Ok(exampleSrv.SayHello(ctx, &proto.HelloRequest{Name: "World"}))
	fmt.Println(result.Message)
}
`

var scriptTsTpl = `/// <reference path="../services/ExampleService.ts" />
import ngrpc from "@ayonli/ngrpc";

ngrpc.runSnippet(async () => {
    const result = await services.ExampleService.sayHello({ name: "World" });
    console.log(result.message);
});
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initiate a new NgRPC project",
	Run: func(cmd *cobra.Command, args []string) {
		tsConfFile := "tsconfig.json"
		confFile := "ngrpc.json"
		protoDir := "proto"
		protoFile := "proto/ExampleService.proto"
		servicesDir := "services"
		scriptsDir := "scripts"

		var goModName = getGoModuleName()
		var entryFile string
		var serviceFile string
		var scriptFile string
		template := cmd.Flag("template").Value.String()

		if template == "go" {
			if goModName == "" {
				fmt.Println("'go.mod' file not found in the current directory")
				return
			} else {
				entryFile = "main.go"
				serviceFile = "services/ExampleService.go"
				scriptFile = "scripts/main.go"
			}
		} else if template == "node" {
			if !util.Exists("package.json") {
				fmt.Println("'package.json' file not found in the current directory")
				return
			} else {
				entryFile = "main.ts"
				serviceFile = "services/ExampleService.ts"
				scriptFile = "scripts/main.ts"
			}
		} else {
			fmt.Printf("template '%s' is not supported\n", template)
			return
		}

		if template == "node" {
			if util.Exists(tsConfFile) {
				fmt.Printf("file '%s' already exists\n", tsConfFile)
			} else {
				os.WriteFile(tsConfFile, []byte(tsConfTpl), 0644)
				fmt.Printf("tsconfig file written to '%s'\n", tsConfFile)
			}
		}

		if util.Exists(confFile) {
			fmt.Printf("file '%s' already exists\n", confFile)
		} else {
			var tpl string

			if template == "go" {
				tpl = confTpl
			} else if template == "node" {
				tpl = strings.Replace(confTpl, `"main.go"`, `"main.ts"`, -1)
			}

			os.WriteFile(confFile, []byte(tpl), 0644)
			fmt.Printf("config file written to '%s'\n", confFile)
		}

		if util.Exists(entryFile) {
			fmt.Printf("entry file '%s' already exists\n", entryFile)
		} else {
			var tpl string

			if template == "go" {
				tpl = strings.Replace(
					entryGoTpl,
					"github.com/ayonli/ngrpc/services",
					goModName+"/services",
					-1)
			} else if template == "node" {
				tpl = entryTsTpl
			}

			os.WriteFile(entryFile, []byte(tpl), 0644)
			fmt.Printf("entry file written to '%s'\n", entryFile)
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

		if util.Exists(scriptsDir) {
			fmt.Printf("path '%s' already exists\n", scriptsDir)
		} else {
			util.EnsureDir(scriptsDir)
			fmt.Printf("path '%s' created\n", scriptsDir)
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
			var tpl string

			if template == "go" {
				tpl = strings.Replace(
					exampleServiceGoTpl,
					"github.com/ayonli/ngrpc/services",
					goModName+"/services",
					-1)
			} else if template == "node" {
				tpl = exampleServiceTsTpl
			}

			os.WriteFile(serviceFile, []byte(tpl), 0644)
			fmt.Printf("example service file written to '%s'\n", serviceFile)
		}

		if util.Exists(scriptFile) {
			fmt.Printf("file '%s' already exists\n", scriptFile)
		} else {
			var tpl string

			if template == "go" {
				tpl = strings.Replace(
					scriptGoTpl,
					"github.com/ayonli/ngrpc/services",
					goModName+"/services",
					-1)
			} else if template == "node" {
				tpl = scriptTsTpl
			}

			os.WriteFile(scriptFile, []byte(tpl), 0644)
			fmt.Printf("script file written to '%s'\n", scriptFile)
		}

		// install dependencies
		var depCmd *exec.Cmd

		if template == "go" {
			protoc() // generate code from proto files

			depCmd = exec.Command("go", "mod", "tidy")
		} else if template == "node" {
			depCmd = exec.Command("npm", "i", "typescript", "tslib", "source-map-support")
		}

		if depCmd != nil {
			depCmd.Stdout = os.Stdout
			depCmd.Stderr = os.Stderr
			err := depCmd.Run()

			if err != nil {
				fmt.Println(err)
				return
			}
		}

		fmt.Println("")
		fmt.Println("All procedures finished, now try the following command to start your first gRPC app")
		fmt.Println("")
		fmt.Println("    ngrpc start")
		fmt.Println("")
		fmt.Println("Then try the following command to check out all the running apps")
		fmt.Println("")
		fmt.Println("    ngrpc list")
		fmt.Println("")
		fmt.Println("Or try the following command to run a script that attaches to the service and get some results")
		fmt.Println("")

		if template == "go" {
			fmt.Println("    ngrpc run scripts/main.go")
		} else if template == "node" {
			fmt.Println("    ngrpc run scripts/main.ts")
		}

		fmt.Println("")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("template", "t", "", `available values are "go" or "node"`)
}

func getGoModuleName() string {
	data, err := os.ReadFile("go.mod")

	if err == nil {
		match := stringx.Match(string(data), `module (\S+)`)

		if match != nil {
			return match[1]
		}
	}

	return ""
}
