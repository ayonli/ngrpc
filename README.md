# gRPC Boot

Make it easy to create clear, expressive and elegant gRPC based applications.

*NOTE: this package uses [@hyurl/grpc-async](https://github.com/hyurl/grpc-async) to make life with*
*gRPC easier.*

*NOTE: The NPM package only contains the minimal file base,*
*[go to GitHub for this Doc](https://github.com/hyurl/grpc-boot) and the related files.*
*By combining these files, this project itself serves as an example of using gRPC Boot in real world.*

## Install

```sh
npm i @hyurl/grpc-boot
```

## First Impression

Take a look at the following config file ([grpc-boot.json](./grpc-boot.json)):

```json
{
    "$schema": "./node_modules/@hyurl/grpc-boot/grpc-boot.schema.json",
    "protoDirs": [
        "./services"
    ],
    "protoOptions": {
        "longs": "String",
        "defaults": true,
        "oneofs": true
    },
    "apps": [
        {
            "name": "example-server",
            "uri": "grpc://localhost:4000",
            "serve": true,
            "services": [
                "services.ExampleService"
            ],
            "stdout": "./out.log"
        },
        {
            "name": "user-server",
            "uri": "grpcs://localhost:4001",
            "serve": false,
            "services": [
                "services.UserService"
            ],
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
            "stdout": "./out.log",
            "ca": "certs/ca.pem",
            "cert": "certs/cert.pem",
            "key": "certs/cert.key"
        }
    ]
}
```

Now, start the apps like this:

```sh
npx tsc && npx grpc-boot start
```

It's just that simple.

### Explanation

- `namespace` This is the directory that stores the service class (`.ts`) files, and is the root
    namespace of the services. Normally, this option is omitted and use `services` by default.
- `entry` The entry file that is used to spawn apps.
    Normally, this property is not required because the CLI command will use the default entry
    file for us.

    If a custom entry file is provided, it's spawned with the arguments `appName`, we can use
    `process.argv[2]` to get the app's name. Please take a look at the example [main.ts](./main.ts).
- `importRoot` Where to begin searching for TypeScript / JavaScript files, the default is `.`. If
    given, we need to set this property the same value as the `outDir` compiler option in the
    `tsconfig.json` file.
- `protoDirs` These directories stores the `.proto` files. Normally, `.proto` files reside with the
    service class (`.ts`) files, but they can be placed in a different place, say a `proto` folder,
    just set this option to the correct directory so the program can find and load them.
- `protoOptions` These options are used when loading the `.proto` files.
- `apps` This property configures the apps that this project serves and connects.
    - `name` The name of the app.
    - `uri` The URI of the gRPC server, supported schemes are `grpc:`, `grpcs:`, `http:`, `https:`
        or `xds:` (make sure package [@grpc/grpc-js-xds](https://www.npmjs.com/package/@grpc/grpc-js-xds)
        is installed).
    - `serve` If this app is served by the gRPC Boot app server. If this property is `false`, that
        means the underlying services are served by another program. As we can see from the above
        example, the `user-server` sets this property to `false`, because it's served in a
        [`golang` program](./main.go). If we take a look at the
        [services.UserService](./services/UserService.ts), we will just see a very simple
        TypeScript file that contains an abstract class.
    - `services` The services served by this app. if we take a look at the
        [services.ExampleService](./services/ExampleService.ts) and the
        [services.PostService](./services/PostService.ts), we will see that they're very simple
        TypeScript class files.
    - `stdout` Log file used for stdout.

    **More Options**

    - `ca` The CA filename when using TLS/SSL.
    - `cert` The certificate filename when using TLS/SSL.
    - `key` The private key filename when using TLS/SSL.
        
        NOTE: We only need a pair of certificates for both the server and the client, since they are
        inside one project, using different certificates makes no sense.
    - `connectTimeout` Connection timeout in milliseconds, the default value is `5_000` ms.
    - `options` Channel options, see https://www.npmjs.com/package/@grpc/grpc-js for more details.
    - `stderr` Log file used for stderr.
    - `entry` The entry file that is used to spawn this app. This option overwrites the one set in
        the head.
    - `env` The environment variables passed to the `entry` file.

With these simple configurations, we can write our gRPC application straightforwardly in a `.proto`
file and a `.ts` file in the `services` directory, without any headache of how to start the server
or connect to the services, all is properly handled internally by the gRPC Boot framework.

## CLI Commands

- `init` initiate a new gRPC project

- `start [app]` start an app or all apps (exclude non-served ones)
    - `app` the app name in the config file

- `restart [app]` restart an app or all apps (exclude non-served ones)
    - `app` the app name in the config file

- `reload [app]` reload an app or all apps
    - `app` the app name in the config file

- `stop [app]` stop an app or all apps
    - `app` the app name in the config file

- `list` list all apps (exclude non-served ones)

### Hot-Reloading

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
inconsistency between the two files and causing program failure. So this package provides the
`reload` command that allows us to manually reload the app when we're done with our changes.

### About Process Management

A server app may be automatically respawned if it is crashed, but this behavior requires at least
one app is still running. If our project only have one app, this feature will not function. 

On the other hand, the CLI tool only works for the app instance, if the process contain other
objects that prevent the process to exit, the `stop` command won't be able to terminate the process.

It's recommended to use external process management tool such as **PM2** in production, which gives
us more control of our program and provides more features such as monitoring. And while using PM2
(or others), we can still use the `reload` command to hot-reload our app after deployed new updates.

## Programmatic API

**service(name: string): ClassDecorator**

This decorator function is used to link the service class to a gRPC service.

- `name` The service name defined in the `.proto` file.

**`App.boot(app?: string): Promise<void>`**

Starts the app programmatically.

- `app` The app's name that should be started as a server. If not provided, the app only connects
    to other servers but not serves as one.

**Example**

```ts
import App from "@hyurl/grpc-boot";

(async () => {
    // This app starts a gRPC server named 'example-server' and connects to all services.
    const serverApp = await App.boot("example-server");
})();

(async () => {
    // This app won't start a gRPC server, but connects to all services.
    const clientApp1 = await App.boot();
})();
```

----

**`app.stop(): Promise<void>`**

Stops the app programmatically.

**Example**

```ts
import App from "@hyurl/grpc-boot";

App.boot("example-server").then(app => {
    process.on("exit", (code) => {
        // Stop the app when the program is issued to exit.
        app.stop().then(() => {
            process.exit(code);
        });
    });
});
```

----

**`app.reload(): Promise<void>`**

Reloads the app programmatically.

This function is rarely used explicitly, prefer to use the CLI `reload` command or
`App.sendCommand("reload")` instead.

----

**`app.onReload(callback: () => void): void`**

Registers a callback to run after the app is reloaded.

**Example**

```ts
import App from "@hyurl/grpc-boot";

App.boot("example-server").then(app => {
    app.onReload(() => {
        // Log the reload event.
        console.info("The app has been reloaded");
    });
});
```

----

**`app.onStop(callback: () => void): void`**

Registers a callback to run after the app is stopped.

**Example**

```ts
import App from "@hyurl/grpc-boot";

App.boot("example-server").then(app => {
    app.onStop(() => {
        // Terminate the process when the app is stopped.
        process.exit(0);
    });
});
```

----

**`App.loadConfig(): Promise<Config>`**

Loads the configurations.

----

**`App.loadConfigForPM2(): Promise<{ apps: any[] }>`**

Loads the configurations and reorganize them so that the same configuration can be used in PM2's
configuration file.

----

**`App.sendCommand(cmd: "reload" | "stop" | "list", app?: string): Promise<void>`**

Sends control command to the apps. This function is mainly used in the CLI tool.

- `cmd`
- `app` The app's name that should received the command. If not provided, the
    command is sent to all apps.

----

**`App.runSnippet(fn: () => void | Promise<void>): Promise<void>`**

Runs a snippet inside the apps context.

This function is for temporary scripting usage, it starts a temporary pure-clients app so we can use
the services as we normally do in our program, and after the main `fn` function is run, the app is
automatically stopped.

- `fn` The function to be run.

**Example**

```ts
import App from "@hyurl/grpc-boot";

App.runSnippet(async () => {
    const post = await services.PostService.getPost({ id: 1 });
    console.log(post);
});
```

## Implement a Service

To allow gRPC Boot to handle the serving and connecting process of our services, we need to
implement our service in a well-designed fashion.

For example, a typical service should be designed like this:

```ts
import { ServiceClient, service } from "@hyurl/grpc-boot";

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
import { ServiceClient, service } from "@hyurl/grpc-boot";

declare global {
    namespace services {
        const UserService: ServiceClient<UserService>;
    }
}

@service("services.github.ayonli.UserService")
export default abstract class UserService {
    // abstract methods...
}
```

## Good Practices

1. The package name of the `.proto` file for services is the same namespace in the `.ts` service
    class files.

    For example:

```proto
// the .proto file
syntax = "proto3";

package services;
```

```ts
// the .ts file
declare global {
    namespace services {

    }
}
```

*NOTE: this rule doesn't apply to the situation when multiple projects are*
*[sharing the same `.proto` files](#sharing-proto-files-across-projects).*

2. The package name / namespace is the directory name that store the `.proto` files and `.ts` files.
    For example, package name `services` uses `./services` directory, and `services.sub` uses
    `./services/sub`. This is required for discovering and importing files.

3. The base name of the `.proto` file (without extension) should have a correspondent `.ts` file (or
    `.d.ts` file). For example, `ExampleService.proto` maps to the `ExampleService.ts` file.

4. The `.proto` file should contain only one service and its name is the same name as the file,
    respectively, the correspondent `.ts` file export the default class with the same name.

    For example

```proto
// the ExampleService.proto file
syntax = "proto3";

package services;

service ExampleService {
    // ...
}
```

```ts
// the ExampleService.ts file
import { ServiceClient } from "@hyurl/grpc-boot"

declare global {
    namespace services {
        const ExampleService: ServiceClient<ExampleService>;
    }
}

export default class ExampleService {

}
```

## Lifecycle Support

The service class served by gRPC Boot application supports lifecycle functions, to use this feature,
simply implement the `LifecycleSupportInterface` for the service class, for example:

```ts
import { LifecycleSupportInterface } from "@hyurl/grpc-boot";

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

## Running the Program in TS-Node

The CLI tools starts the program either with `node` or `ts-node` according to the entry file. If the
entry file's extension is `.js`, it spawn the process via `node`, which means the source code (in
TypeScript) needs to be transpiled into JavaScript first in order to be run (the default behavior).
If the filename ends with `.ts`, it load the program via `ts-node`, which allow TypeScript code run
directly in the program.

By default, gRPC Boot app uses a default entry file compiled in JavaScript, which means our code
needs to be transpiled as well. To use `ts-node` running TypeScript, we need to provide a custom
entry file, just like this.

```json
{
    "package": "services",
    "entry": "./main.ts",
    // ...
}
```

Moreover, instead of giving the extension name, we can omit it (for example `./main`) and allow the
CLI tool to determine whether to use `node` or `ts-node` according the file presented. If `main.js`
is presented, `node` is used, otherwise, `ts-node` is used.

## Load Balancing and Routing

If a service is served in multiple apps, gRPC Boot uses a client-side load balancer to connect to it,
the load balancer is configured with a custom routing resolver which allows us redirect traffic
according to the message we sent when calling RPC functions.

To use this feature, define the request message that extends / augments the interface
`RoutableMessageStruct`, it contains a `route` key that can be used in the internal client load
balancer. When the client sending a request which implements this interface, the program will
automatically route the traffic to a certain server evaluated by the `route` key, which can be set
in the following forms:

- a URI that matches the ones that set in the config file;
- an app's name that matches the ones that set in the config file;
- if none of the above matches, use the hash algorithm on the `route` value;
- if `route` value is not set, then the default round-robin algorithm is used for routing.

For Example:

```proto
// the .proto file

message RequestMessage = {
    string route = 1;
    // other fields
};
```

```ts
// the .ts file
import { RoutableMessageStruct } from "@hyurl/grpc-boot";

export interface RequestMessage extends RoutableMessageStruct {
    // other fields
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

The following app do not serve, but connects to all the services according to the configuration file.
We can do all the stuffs provided by GoRPC in the web server as we would in the RPC server,
because all the differences between the gRPC client and the gRPC server are hidden behind the scene.

```ts
import App from "@hyurl/grpc-boot";

(async () => {
    const app = await App.boot();
})()
```

## 0-Services App

Apart from the unnamed app, an app can be configured with `serve: true` but no services, such an app
does not actually start the gRPC server neither consume the port. But such an app can be used, say,
to start a web server, which connects to the gRPC services and uses the facility this package
provides, such as the CLI tool and the reloading hook.

For example:

```json
// grpc-boot.json
{
    // ...
    "entry": "main", // need a custom entry file
    "apps": [
        {
            "name": "web-server",
            "uri": "http://localhost:4000",
            "serve": true,
            "services": [] // leave this blank
        },
        // ...
    ]
}
```

```ts
// main.ts
import App, { Config } from "@hyurl/grpc-boot";
import * as http from "http";
import * as https from "https";
import * as fs from "fs/promises";

if (require.main?.filename === __filename) {
    (async () => {
        const appName = process.argv[2];

        const app = await App.boot(appName);
        let httpServer: http.Server;
        let httpsServer: https.Server;
        
        if (appName === "web-server") {
            const conf = await App.loadConfig();
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

## Good Practices

In order to code a clear, expressive and elegant gRPC based application, apart from the features
that gRPC Boot provides, we can order our project by performing the following steps.

1. Create a `proto` folder to store all the `.proto` files in one place.

2. Create a `services` folder for all the service files, the namespace of those files should be
    the same as the folder's name (which is also `services`). Sub-folders and sub-namespaces are
    also supported.

3. Design the `.proto` files with a reasonable scoped package name, don't just name it `services`, 
    instead, name it something like `services.[website].[provider]`, the `.proto` files should be
    shared and reused across different projects, using a long name to prevent collision and provide
    useful information about the services. Respectively, the directory path should reflect the
    package name. See the [proto](./proto) files of this project as examples.

4. Use the same file structures and symbol names (as possible as we can) in the class files to
    reflect the ones in the `.proto` files, create a consistent development experience.
