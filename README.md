# NgRPC

Make it easy to create clear, expressive and elegant gRPC based microservices.

This package is written in and for both Node.js and Golang.

*Windows OS is not yet supported but on the way to go.*

## Install

**In Node.js**

```sh
npm i @ayonli/ngrpc
```

**In Golang**

```sh
go get github.com/ayonli/ngrpc
```

### Install the CLI tool

The CLI tool is written in Golang, so we have to install it via `go install` command:

```sh
go build github.com/ayonli/ngrpc/cli/ngrpc
```

## A Simple Example

First, take a look at this configuration ([ngrpc.json](./ngrpc.json)):

```json
{
    "$schema": "./ngrpc.schema.json",
    "protoPaths": [
        "proto"
    ],
    "apps": [
        {
            "name": "example-server",
            "uri": "grpc://localhost:4000",
            "serve": true,
            "services": [
                "services.ExampleService"
            ],
            "entry": "entry/main.ts", // alternatively we can use `entry/main.go` instead.
            "stdout": "out.log"
        },
        {
            "name": "user-server",
            "uri": "grpcs://localhost:4001",
            "serve": true,
            "services": [
                "services.UserService"
            ],
            "entry": "entry/main.go",
            "stdout": "out.log",
            "ca": "certs/ca.pem",
            "cert": "certs/cert.pem",
            "key": "certs/cert.key"
        },
        {
            "name": "post-server",
            "uri": "grpcs://localhost:4002",
            "serve": true,
            "services": [
                "services.PostService"
            ],
            "entry": "entry/main.ts",
            "stdout": "out.log",
            "ca": "certs/ca.pem",
            "cert": "certs/cert.pem",
            "key": "certs/cert.key"
        }
    ]
}
```

We have two different entry files here, let's dig in each of them.

[main.ts](./entry/main.ts)

```ts
import ngrpc from "@ayonli/ngrpc";

if (require.main?.filename === __filename) {
    const appName = process.argv[2];

    ngrpc.boot(appName).then(app => {
        process.send?.("ready"); // for PM2 compatibility
        app.waitForExit();
    }).catch(err => {
        console.error(err);
        process.exit(1);
    });
}
```

[main.go](./entry/main.go)

```go
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
```

### Explanation

- `protoPaths` The directories that stores the `.proto` files. Normally, `.proto` files are stored
    in the `proto` folder.

    TIP: don't forget add `--proto_path=${workspaceRoot}/proto` to the `protoc.options` in VSCode's
    settings in order for the **vscode-proto3** plugin to work properly.
- `apps` This property configures the apps that this project serves and connects.
    - `name` The name of the app.
    - `uri` The URI of the gRPC server, supported schemes are `grpc:`, `grpcs:`, `http:`, `https:`
        or `xds:` (in Node.js, make sure package
        [@grpc/grpc-js-xds](https://www.npmjs.com/package/@grpc/grpc-js-xds) is installed).
    - `serve` If this app is served by the NgRPC app server. If this property is `false`, that
        means the app is served by other programs and we just connect to it.
    - `services` The services served by this app.
    - `entry` The entry file used to spawn apps.
        - During development, the entry filename shall be suffixed either by `.ts` or `.go`, when
        running the program, NgRPC automatically compiles the file when needed.

        - In production, the entry filename shall the compiled file's name, which is suffixed by `.js`
        or has no suffix at all (for Golang, or `.exe` in Windows).

        The program is spawned with the argument `appName`, in Node.js, we use `process.argv[2]` to
        retrieve it, and in Golang, we use `os.Args[1]`.
    - `stdout` Log file used for stdout.
    
    **More Options for `apps`**
    
    - `ca` The CA filename when using TLS/SSL.
    - `cert` The certificate filename when using TLS/SSL.
    - `key` The private key filename when using TLS/SSL.
      
        NOTE: We only need a pair of certificates for both the server and the client, since they are
        inside one project, using different certificates makes no sense.
    - `stderr` Log file used for stderr. If omitted and `stdout` is set, the program uses `stdout`
        for `stderr` as well.
    - `env` Additional environment variables passed to the `entry` file.

**More Top Options**

- `namespace` This is the root namespace of the services in Node.js, and the directory that
    stores the service class (`.ts`) files. Normally, this option is omitted and use `services` by
    default.
- `importRoot` Where to begin searching for TypeScript / JavaScript files, the default is `.`. If
    given, there are two rules for setting this option:
    
    - During development, if we use a source directory, say `src` (respectively, set
        `compilerOptions.rootDir` to `src` in `tsconfig.json`), then this option should be set to
        `src` as well.
    - In production, if we compile our program to a build directory, say `dist` (respectively, set
        `compilerOptions.outDir` to `dist` in `tsconfig.json`), then this option should be set to
        `dist` as well.
    
    This options is also used for generating Golang code from the `.proto` files. If we set a `src`
    directory, the code will be generated into this directory as well.
- `protoOptions` These options are used when loading the `.proto` files in Node.js. Check
    [ngrpc.schema.json](./ngrpc.schema.json) for more details.

In Node.js, services are automatically discoverd and imported when the program starts, in Golang, we
import the `services` package and name it `_` for its side-effect which registers the services.

Then we use the `ngrpc.boot()` / `ngrpc.Boot()` function to initiate the app by the given name, it
initiates the server (if served) and client connections, prepares the services ready for use.

Next we use the `app.waitForExit()` / `app.WaitForExit()` function to wait for the interrupt / exit
signal from the system for a graceful shutdown. In Golang, this function also keeps the program
running and prevent premature exit.

With these simple configurations, we can write our gRPC application straightforwardly in `.proto`
files and `.ts` or `.go` files, without any headache of when and how to start the server or
connect to the services, all is properly handled behind the scene.

## CLI Commands

- `ngrpc init [flags]` initiate a new NgRPC project
    - `-t --template <string>` available values are "go" or "node"

    TIP: we can run this command twice with different template for the setup for both languages,
    existing files will be untouched.
- `ngrpc start [app]` start an app or all apps (exclude non-served ones)
    - `app` the app name in the config file

- `ngrpc restart [app]` restart an app or all apps (exclude non-served ones)
    - `app` the app name in the config file

- `ngrpc reload [app]` hot-reload an app or all apps
    - `app` the app name in the config file

    NOTE: only Node.js supports hot-reloading, Golang programs just reply that they don't support
    this feature.
- `ngrpc stop [app]` stop an app or all apps
    - `app` the app name in the config file

- `ngrpc list` or `ngrpc ls` list all apps (exclude non-served ones)

- `ngrpc run <filename> [args...]` runs a script file that attaches to the services, can be either
    Golang (`.go`) or Node.js (`.ts`) programs.

- `ngrpc protoc` generate golang program files from the proto files.

    NOTE: this command is not used if our project only contains Node.js programs.

- `ngrpc host [flags]` start the host server in standalone mode
    - `--stop` stop the host server

    NOTE: when `start` command is issued, the host server will be automatically started. The `host`
    command is used when our program isn't started by the `start` command and we need the
    functionalities that **ngrpc** provides. For examples, we start our program via **PM2** and we
    still need the `reload` command to function once we deployed new updates.

### Hot-Reloading (only Node.js)

After we've modified our source code (or recompiled), the `.proto` files, or the config file, we can
use the `reload` command to hot-reload our apps without restarting the process.

When the command is issued, the application will scan the imported service files and their
dependencies (exclude the ones in `node_modules`), and reload them all at once. Since this procedure
doesn't restart the process, all context stores in the global scope are still available.
Hot-reloading is much faster than restarting the whole program, the clients will experience
0-downtime of our services.

It's important to point out, though, that the hot-reloading model this package uses only supports
services and their dependencies, any other code, for example, the change of the entry file, will not
join the reloading process and cannot be hot-reloaded, if such changes are made, a full restart is
needed for the new code to run.

**Why not auto-reload when the file is changed?**

gRPC uses the `.proto` file for definition and the `.ts` file for implementation, it's hard to keep
track on both files at the same time. If reload immediately after a file is changed, there may be
inconsistency between the two files and causing the program to fail. So this package provides the
`reload` command that allows us to manually reload the app when we're done with our changes.

### About Process Management

This package uses a host-guest model for process management. When using the `start` command to start
the app, the CLI tool also starts a host server to hold communication between apps, the host is
responsible to accept commands sent by the CLI tool and distribute them to the app.

When an app crashes, the host server is also responsible for re-spawning it, this feature guarantees
that our app is always online. Except when the host server is running in standalone mode, at which
the app should be re-spawned by the external process management like PM2.

Moreover, that the CLI tool only works for the app instance, if the process contains other logics
that prevent the process to exit, the `stop` command will not be able to terminate the process, in
such case, a hard kill is required.

## Implement a Service

To allow NgRPC to handle the serving and connecting process of our services, we need to
implement our service in a well-designed fashion.

For example, a typical service should be designed like this:

### In Node.js

```ts
import { ServiceClient, service } from "@ayonli/ngrpc";

declare global {
    namespace services {
        const ExampleService: ServiceClient<ExampleService>;
    }
}

@service("services.ExampleService")
export default class ExampleService {
    // methods and private fields...
}
```

If this is a client-side service representation (only for referencing), it should be defined as an
abstract class, like this:

```ts
import { ServiceClient, service } from "@ayonli/ngrpc";

declare global {
    namespace services {
        const UserService: ServiceClient<UserService>;
    }
}

@service("github.ayonli.ngrpc.services.UserService")
export default abstract class UserService {
    // abstract methods...
}
```

### In Golang

```go
type ExampleService struct {
    // A service to be served need to embed the UnimplementedServiceServer.
    proto.UnimplementedExampleServiceServer
}
```

For NgRPC, a client-side service representation struct is needed as well:

```go
// A pure client service is an empty struct, which is only used for referencing to the service.
type ExampleService {}
```

#### func init

In each service file, we need to define a `init` function to use the service:

```go
func init() {
    ngrpc.Use(&ExampleService{})
}
```

#### func Serve

For a service in order to be served, a `Serve()` method is required in the service struct:

```go
func (self *ExampleService) Serve(s grpc.ServiceRegistrar) {
    proto.RegisterExampleServiceServer(s, self)

    // other initiations, like establishing database connections
}
```

#### func Connect

All services (server-side and client-side) must implement the `Connect()` method in order to be
connected:

```go
func (self *Service) Connect(cc grpc.ClientConnInterface) proto.ExampleServiceClient {
    return proto.NewExampleServiceClient(cc)
}
```

#### func GetClient

The service may implement a `GetClient()` which can be used to reference the service client
in a more expressive way:

```go
func (self *ExampleService) GetClient(route string) (proto.ExampleServiceClient, error) {
    return ngrpc.GetServiceClient(self, route)
}
```

## Lifecycle Support

The service class served by NgRPC application supports lifecycle functions, to use this feature,
in Node.js, simply implement the `LifecycleSupportInterface` for the service class, for example:

```ts
import { LifecycleSupportInterface, service } from "@ayonli/ngrpc";

@service("services.ExampleService")
export default class ExampleService implements LifecycleSupportInterface {
    async init(): Promise<void> {
        // When the service is loaded (or reloaded), the `init()` method will be automatically
        // called, we can add some async logic inside it, for example, establishing database
        // connection, which is normally not possible in the default `constructor()` method
        // since it doesn't support asynchronous codes.
    }

    async destroy(): Promise<void> {
        // When the app is about to stop, or the service is about to be reloaded, the `destroy()`
        // method will be called, which gives the ability to clean up and release resource.
    }
}
```

In Golang, we use the `Serve()` method for additional setup, and the `Stop()` method for teardown.

```go
func (self *ExampleService) Serve(s grpc.ServiceRegistrar) {
    proto.RegisterExampleServiceServer(s, self)

    // other initiations, like establishing database connections
}

func (self *ExampleService) Stop() {
    // release database connections, etc.
}
```

## Dependency Injection

**In Node.js**

Just add a private property in the class that points to another service, like this:

```ts
@service("github.ayonli.ngrpc.services.PostService")
export default class PostService {
    private userSrv = services.UserService;
    // other private properties...

    async getPost(query: PostQuery): Promise<Post> {
        const post = this.postStore?.find(item => item.id === query.id);

        if (post) {
            // ---- highlight ----
            const author = await this.userSrv.getUser({ id: post.author, });
            // ---- highlight ----

            return { ...post, author };
        } else {
            throw new Error(`Post ${query.id} not found`);
        }
    }
}
```

**In Golang**

Just add an exported field that points to another service, like this:
```go
type UserService struct {
    proto.UnimplementedUserServiceServer
    PostSrv   *PostService // set as exported field for dependency injection
    // other non-exported fields...
}

func (self *UserService) GetMyPosts(ctx context.Context, query *proto.UserQuery) (*proto.PostQueryResult, error) {
    return goext.Try(func() *services_proto.PostQueryResult {
        user := goext.Ok(self.GetUser(ctx, query))

        // ---- highlight ----
        ins := goext.Ok(self.PostSrv.GetClient(user.Id))
        // ---- highlight ----

        res := goext.Ok(ins.SearchPosts(ctx, &services_proto.PostsQuery{Author: &user.Id}))

        return (*services_proto.PostQueryResult)(res)
    })
}
```

## Load Balancing and Routing

If a service is served in multiple apps, NgRPC uses a client-side load balancer to connect to it,
the load balancer is configured with a custom routing resolver which automatically redirect traffic
for us.

There are three algorithms used based on the `route`:

1. When `route` is not empty:
    - If it matches one of the name or URI of the apps, the traffic is routed to that app directly.
    - Otherwise the program hashes the route string against the apps and match one by the mod value
        of `hash % active_nodes`.
2. When `route` is empty, the program uses *round-robin* algorithm against the active nodes.

**In Node.js**

To use this feature, define the request message that extends / augments the interface
`RoutableMessageStruct`, it contains a `route` key that can be used in the internal client load
balancer. When the client sending a request which implements this interface, the program will
automatically route the traffic to a certain server evaluated by the `route` key.

**In Golang**

To use this feature, we need to provide the `route` argument when calling the
`ngrpc.GetServiceClient()` function or the `GetClient()` of the service struct.

For Example:

**The `.proto` File**

```proto
message RequestMessage = {
    string route = 1;
    // other fields
};
```

**In Node.js**

```ts
// the .ts file
import { RoutableMessageStruct } from "@ayonli/ngrpc";

export interface RequestMessage extends RoutableMessageStruct {
    // other fields
}
```

**In Golang**

```go
func main() {
    msg := &proto.RequestMessage{
        Route: "route key"
        // other fields
    }
    ins := ngrpc.GetServiceClient(&services.SomeService{}, msg.Route)
}
```

Apart from the client-side load balancing, server-side load balancing is automatically supported by
gRPC, either by reverse proxy like NGINX or using the `xds:` protocol for Envoy Proxy.

## Unnamed App

It it possible to boot an app without providing the name, such an app will not start the server, but
only connects to the services. This is useful when we're using gRPC services in a frontend server,
for example, a web server, which only handles client requests and direct calls to the backend gRPC
services, we need to establish connection between the web server and the RPC servers, but we don't
won't to serve any service in the web server.

The following apps do not serve, but connect to all the services according to the configuration file.
We can do all the stuffs provided by NgRPC in the web server as we would in the RPC server,
because all the differences between the gRPC client and the gRPC server are hidden behind the scene.

**In Node.js**

```ts
import ngrpc from "@ayonli/ngrpc";

(async () => {
    const app = await ngrpc.boot();
})()
```

**In Golang**

```go
import "github.com/ayonli/ngrpc"

func main() {
    app, err := ngrpc.Boot("")
}
```

## 0-Services App

Apart from the unnamed app, an app can be configured with `serve: true` but no services, such an app
does not actually start the gRPC server neither consume the port. But such an app can be used, say,
to start a web server, which connects to the gRPC services and uses the facility this package
provides, such as the CLI tool and the hot-reloading model.

For example:

```json
// ngrpc.json
{
    // ...
    "apps": [
        {
            "name": "web-server",
            "uri": "http://localhost:4000",
            "serve": true,
            "services": [] // leave this blank
            // ...
        },
        // ...
    ]
}
```

**In Node.js**

```ts
// main.ts
import ngrpc, { Config } from "@ayonli/ngrpc";
import * as http from "http";
import * as https from "https";
import * as fs from "fs/promises";

if (require.main?.filename === __filename) {
    (async () => {
        const appName = process.argv[2];

        const app = await ngrpc.boot(appName);
        let httpServer: http.Server;
        let httpsServer: https.Server;
        
        if (appName === "web-server") {
            const conf = await ngrpc.loadConfig();
            const _app = conf.apps.find(app => app.name === appName) as Config["apps"][0];
            let { protocol, port } = new URL(_app.uri);

            if (protocol === "https:") {
                port ||= "443";
                httpsServer = https.createServer({
                    cert: await fs.readFile(_app.cert as string),
                    key: await fs.readFile(_app.key as string),
                }, (req, res) => {
                    // ...
                }).listen(port);
            } else if (protocol === "http:") {
                port ||= "80";
                httpServer = http.createServer((req, res) => {
                    // ...
                }).listen(port);
            }
        } 

        app.onReload(() => {
            // do some logic to reload the HTTP(S) server
        });
        app.onStop(() => {
            httpServer?.close();
            httpsServer?.close();
        });

        process.send?.("ready");
    })().catch(err => {
        console.error(err);
        process.exit(1);
    });
}
```

**In Golang**

It's similar to the Node.js version, except it uses Golang's `net/http` package and it's own way.

## Running Scripts

Sometimes it's just useful that we can write a simple script program that connects to the services
and call their methods, for whatever the reason is, NgRPC makes this very easy for us.

**In Node.js**

```ts
/// <reference path="./services/ExampleService.ts" />
import ngrpc from "@ayonli/ngrpc";

ngrpc.runSnippet(async () => {
    const result = await services.ExampleService.sayHello({ name: "World" });
    console.log(result.message);
});
```


**In Golang**

```go
package main

import (
    "context"
    "fmt"

    "github.com/ayonli/goext"
    "github.com/ayonli/ngrpc"
    "github.com/ayonli/ngrpc-test/services"
    "github.com/ayonli/ngrpc-test/services/proto"
)

func main() {
    done := ngrpc.ForSnippet()
    defer done()

    ctx := context.Background()
    exampleSrv := goext.Ok(ngrpc.GetServiceClient(&services.ExampleService{}, ""))

    result := goext.Ok(exampleSrv.SayHello(ctx, &proto.HelloRequest{Name: "World"}))
    fmt.Println(result.Message)
}
```

## Good Practices

In order to code a clear, expressive and elegant gRPC based application, apart from the features
that NgRPC provides, we can order our project by performing the following steps.

1. Uses the `proto` folder to store all the `.proto` files in one place (by default).

2. Uses the `services` folder for all the service files (by default), the namespace / package of
    those files should be the same as the folder's name (which is also `services`).
    
    NOTE: although sub-folders and sub-namespaces / sub-packages are supported, it's a little tricky
    in Golang, try to prevent this as we can.

3. Design the `.proto` files with a reasonable scoped package name, don't just name it `services`, 
    instead, name it something like `[org].[repo].services`, `.proto` files should be shared and
    reused across different projects, using a long name to prevent collision and provide useful
    information about the service. Respectively, the directory path should reflect the package name.
    See the [proto](./proto) files of this project as examples.

4. **In Node.js**, use the same file structures and symbol names (as possible as we can) in the
    class files to reflect the ones in the `.proto` files, create a consistent development
    experience.

    **In Golang**, Always implement the `GetClient()` method in the service and use an exported
    field in the service struct to reference to each other (for dependency injection).


## Programmatic API

For Node.js, see [api-node.md](./api-node.md).

For Golang, see [the package detail](https://pkg.go.dev/github.com/ayonli/ngrpc).
