## Programmatic API for Node.js

**service(name: string): ClassDecorator**

This decorator function is used to link the service class to a gRPC service.

- `name` The service name defined in the `.proto` file.

**`ngrpc.start(appName?: string): Promise<RpcApp>`**

Initiates an app by the given name and loads the config file, it initiates the server (if served)
and client connections, prepares the services ready for use.

*NOTE: There can only be one named app running in the same process.*

- `appName` The app's name that should be started as a server. If not provided, the app only
    connects to other servers but not serves as one.

**Example**

```ts
import ngrpc from "@ayonli/ngrpc";

(async () => {
    // This app starts a gRPC server named 'example-server' and connects to all services.
    const serverApp = await ngrpc.start("example-server");
})();

(async () => {
    // This app won't start a gRPC server, but connects to all services.
    const clientApp = await ngrpc.start();
})();
```

----

**`ngrpc.startWithConfig(appName: string | null, config: Config): Promise<RpcApp>`**

Like `start()` except it takes a config argument instead of loading the config file.

----

**`app.stop(): Promise<void>`**

Closes client connections and stops the server (if served), and runs any `destroy()` method in the
bound services.

**Example**

```ts
import ngrpc from "@ayonli/ngrpc";

ngrpc.start("example-server").then(app => {
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

This function is rarely used explicitly, prefer the CLI `reload` command instead.

----

**`app.waitForExit(): void**

Listens for system signals to exit the program.

This method calls the `stop()` method internally, if we don't use this method, we need to
call the `stop()` method explicitly when the program is going to terminate.

----

**`app.onStop(callback: () => void): void`**

Registers a callback to run after the app is stopped.

**Example**

```ts
import ngrpc from "@ayonli/ngrpc";

ngrpc.start("example-server").then(app => {
    app.onStop(() => {
        // Terminate the process when the app is stopped.
        process.exit(0);
    });
});
```

----

**`app.onReload(callback: () => void): void`**

Registers a callback to run after the app is reloaded.

**Example**

```ts
import ngrpc from "@ayonli/ngrpc";

ngrpc.start("example-server").then(app => {
    app.onReload(() => {
        // Log the reload event.
        console.info("The app has been reloaded");
    });
});
```

----

**`ngrpc.loadConfig(): Promise<Config>`**

Loads the configurations.

----

**`ngrpc.loadConfigForPM2(): { apps: any[] }`**

Loads the configurations and reorganizes them so that the same configuration can be used in PM2's
configuration file.

----

**`ngrpc.getAppName(): string`**

Retrieves the app name from the `process.argv`.

----

**`ngrpc.getServiceClient<T extends object>(serviceName: string, route?: string): ServiceClient<T>**

Returns the service client by the given service name.

- `route` is used to route traffic by the client-side load balancer.

----

**`ngrpc.runSnippet(fn: () => void | Promise<void>): Promise<void>`**

Runs a snippet inside the apps context.

This function is for temporary scripting usage, it starts a temporary pure-clients app so we can use
the services as we normally do in our program, and after the main `fn` function is run, the app is
automatically stopped.

- `fn` The function to be run.

**Example**

```ts
import ngrpc from "@ayonli/ngrpc";

ngrpc.runSnippet(async () => {
    const post = await services.PostService.getPost({ id: 1 });
    console.log(post);
});
```
