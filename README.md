# gRPC Boot

Make it easy to create standalone and elegant gRPC based applications.

## Install

```sh
npm i @hyurl/grpc-boot
```

## First Impression

Take a look at the following config file ([grpc-boot.json](./grpc-boot.json)):

```json
{
    "$schema": "./node_modules/@hyurl/grpc-boot/grpc-boot.schema.json",
    "package": "services",
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
            "uri": "grpc://localhost:4001",
            "serve": false,
            "services": [
                "services.UserService"
            ]
        },
        {
            "name": "post-server",
            "uri": "grpc://localhost:4002",
            "serve": true,
            "services": [
                "services.PostService"
            ],
            "stdout": "./out.log"
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

- `package` This is the directory that stores the service class files, and is the root namespace of
    the services, as well as the package name in the `.proto` files.
- `entry` The entry file that is used to spawn apps.
    Normally, this property is not required because the CLI command will use the default entry
    file for us.

    If a custom entry file is provided, it's spawned with the arguments `appName [config]`, we
    can use `process.argv[2]` to get the app's name and `process.argv[3]` to get the config
    filename (if provided). Please take a look at the example [main.ts](./main.ts).
- `importRoot` Where to begin searching for TypeScript / JavaScript files, the default is `.`. If
    given, we need to set this property the same value as the `outDir` compiler option in the
    `tsconfig.json` file.
- `protoDirs` These directories stores the `.proto` files, normally, they reside with the service
    class files, so we set to `services` as well.
- `protoOptions` These options are used when loading the `.proto` files.
- `apps` This property configures the apps that this project serves and connects.
    - `name` The name of the app.
    - `uri` The URI of the gRPC server, supported schemes are `grpc:`, `grpcs:` or `xds:`.
    - `serve` If this app is served by the gRPC Boot app server. If this property is `false`, that
        means the underlying services are served by another program. As we can see from the above
        example, the `user-server` sets this property to `false`, because it's served in a
        [`golang` program](./main.go). If we take a look at the
        [services.UserService](./services/UserService.d.ts), we will just see a very simple
        TypeScript declaration file.
    - `services` The services served by this app. if we take a look at the
        [services.ExampleService](./services/ExampleService.ts) and the
        [services.PostService](./services/PostService.ts), we will see that they're very simple
        TypeScript class files.
    - `stdout` Log file used for stdout.

    **More Options**

    - `cert` The certificate filename when using SSL.
    - `key` The private key filename when using SSL.
    - `connectTimeout` Connection timeout is milliseconds, the default value is `5_000` ms.
    - `options` Channel options, see https://www.npmjs.com/package/@grpc/grpc-js for more details.
    - `stderr` Log file used for stderr.
    - `env` The environment variables passed to the `entry` file.

With these simple configurations, we can write our gRPC application straightforwardly in a `.proto`
file and a `.ts` file in the `services` directory, without any headache of how to start the server
or connect to the services, all is properly handled internally by the gRPC Boot framework.

**NOTE: this package uses [@hyurl/grpc-async](https://github.com/hyurl/grpc-async) to make life with**
**gRPC easier.**

## CLI Commands

- `init [options] [package]` initiate a new gRPC project
    - `options`
        - `-c, --config <filename>` create a custom config file
    - `package` The package name / root namespace of the services, default 'services'

- `start [options] [app]` start a gRPC app or all apps (exclude non-served ones)
    - `options`
        - `-c, --config <filename>` use a custom config file
    - `app` the app name in the config file

- `restart [options] [app]` restart a gRPC app or all gRPC apps (exclude non-served ones)
    - `options`
        - `-c, --config <filename>` use a custom config file
    - `app` the app name in the config file

- `reload [options] [app]` reload a gRPC app or all gRPC apps
    - `options`
        - `-c, --config <filename>` use a custom config file
    - `app` the app name in the config file

- `stop [options] [app]` stop a gRPC or all gRPC apps
    - `options`
        - `-c, --config <filename>` use a custom config file
    - `app` the app name in the config file

- `list [options]` list all gRPC apps (exclude non-served ones)
    - `options`
        - `-c, --config <filename>` use a custom config file

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
one server app is still running. If our project only have one app that serves as a server, this
feature will not function. 

On the other hand, the CLI tool only works for the app instance, if the process contain other
objects that prevent the process to exit, the `stop` command won't be able to terminate the process.

It's recommended to use external process management tool such as **PM2** in production, which gives
us more control of our program and provides more features such as monitoring. And while using PM2
(or others), we can still use the `reload` command to hot-reload our app after deployed new updates.

## Programmatic API

**`App.boot(app?: string, config?: string): Promise<void>`**

Starts the app programmatically.

- `app` The app's name that should be started as a server. If not provided, the app only  connects
    to other servers but not serves as one.
- `config` Use a custom config file.

**Example**

```ts
import App from "@hyurl/grpc-boot";

(async () => {
    // This app starts a gRPC server named 'example-server' and connects to all services.
    const serverApp1 = await App.boot("example-server");

    // This app starts a gRPC server with a custom config file.
    const serverApp2 = await App.boot("example-server", "my.config.json");
})();

(async () => {
    // This app won't start a gRPC server, but connects to all services.
    const clientApp1 = await App.boot();

    // This app connects to all services with a custom config file.
    const clientApp2 = await App.boot(null, "my.config.json");
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

**`App.loadConfig(config?: string): Config`**

Loads the configurations.

- `config` Use a custom config file.

----

**`App.loadConfigForPM2(config?: string): { apps: any[] }`**

Loads the configurations and reorganize them so that the same configuration can be used in PM2's
`ecosystem.config.js` file.

- `config` Use a custom config file.

----

**`App.sendCommand(cmd: "reload" | "stop" | "list", app?: string, config?: string): Promise<void>`**

Sends control command to the gRPC apps. This function is mainly used in the CLI tool.

- `cmd`
- `app` The app's name that should received the command. If not provided, the
    command is sent to all apps.
- `config` Use a custom config file.

----

**`App.runSnippet(fn: () => void | Promise<void>, config?: string): Promise<void>`**

Runs a snippet inside the gRPC apps context.

This function is for temporary scripting usage, it starts a temporary pure-clients app so we can use
the services as we normally do in our program, and after the main `fn` function is run, the app is
automatically stopped.

- `fn` The function to be run.
- `config` Use a custom config file.

**Example**

```ts
import App from "@hyurl/grpc-boot";

App.runSnippet(async () => {
    const post = await services.PostService.getPost({ id: 1 });
    console.log(post);
});
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
        // When the app starts and the service is loaded (or reloaded), the `init()` method will be
        // automatically called, we can add some async logic inside it, for example, establishing
        // database connection, which is normally not possible in the default `constructor()` method
        // since it doesn't support asynchronous codes.
    }

    async destroy(): Promise<void> {
        // when the app is about to stop, or the service is about to be reloaded, the `destroy()`
        // method will be called, which gives the ability to clean up and release resource.
    }
}
```

## Routing According to the Message

If a service is served in multiple apps, gRPC Boot uses a client-side load balancer to connect to it,
the load balancer is configured with a custom routing resolver which allows us redirect traffic
according to the message we sent when calling RPC functions.

To use this feature, define the request message that extends / augments the interface
`RoutableMessageStruct`, it contains a `route` key that can be used in the internal client load
balancer. When the client sending a request which implements this interface, the program will
automatically route the traffic to a certain server evaluated by the `route` key, which can be set
in the following forms:

- a URI or address that corresponds to the ones that set in the config file;
- an app's name that corresponds to the ones that set in the config file;
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

Moreover, instead of given the extension name, we can omitted (for example `./main`) and allow the
CLI tool to determine whether to use `node` or `ts-node` according the file presented. If `main.js`
is presented, `node` is used, otherwise, `ts-node` is used.

## Multi-Config Project

It's possible to define a project with multiple gRPC Boot configurations, just pass the custom
config filename everywhere we need. This suits the scenario that the gRPC servers from other
projects uses other package names that is different from ours (commonly `services`).

For example, another project uses the package name `helloworld`, and we use the `GreeterService`
from it. To do this, we can create a custom config file `helloworld.grpc-boot.json` like this:

```json
{
    "package": "helloworld",
    "protoDirs": ["helloworld"],
    "protoOptions": {
        "longs": "String",
        "defaults": true,
        "oneofs": true
    },
    "apps": [
        {
            "name": "greeter-server",
            "uri": "grpc://greeter-server:4000",
            "services": [
                "helloworld.GreeterService"
            ]
        },
    ]
}
```

Then create a folder named `helloworld`, inside it, we create a TypeScript declaration
`GreeterService.d.ts`:

```ts
import { ServiceClient } from "@hyurl/grpc-boot";

declare global {
    namespace helloworld {
        const GreeterService: ServiceClient<GreeterService>;
    }
}

// declare message types ...

export default class GreeterService {
    // declare methods ...
}
```

Then in our entry file, add the following code:

```ts
await App.boot(null, "helloworld.grpc-boot.json");
```
