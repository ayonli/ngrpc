import * as fs from "fs/promises";
import * as path from "path";
import { Options as ProtoOptions, loadSync } from "@grpc/proto-loader";
import {
    ChannelCredentials,
    ChannelOptions,
    GrpcObject,
    Server,
    ServerCredentials,
    ServiceClientConstructor,
    connectivityState,
    credentials,
    loadPackageDefinition
} from "@grpc/grpc-js";
import { serve, unserve, connect, ServiceClient } from "@ayonli/grpc-async";
import get = require("lodash/get");
import set = require("lodash/set");
import isEqual = require("lodash/isEqual");
import hash = require("string-hash");
import {
    absPath,
    exists,
    isTsNode,
    sServiceName,
    timed
} from "./util";
import { findDependencies } from "require-chain";
import { createChainingProxy } from "./nsp";
import { Guest } from "./host/guest";

export type { ServiceClient };

let xdsEnabled = false;

export type ServiceClass = (new () => any) & {
    getInstance?(): any;
    [sServiceName]: string;
};

export interface App {
    /** The name of the app. */
    name: string;
    /**
     * The URI of the gRPC server, supported schemes are `grpc:`, `grpcs:`, `http:`, `https:` or
     * `xds:`.
     */
    uri: string;
    /** If this app can be served by as the gRPC server. */
    serve?: boolean;
    /** The services served by this app. */
    services?: string[];
    /** The CA filename when using TLS/SSL. */
    ca?: string;
    /** The certificate filename when using TLS/SSL. */
    cert?: string;
    /** The private key filename when using TLS/SSL. */
    key?: string;
    stdout?: string;
    stderr?: string;
    entry?: string;
    env?: { [name: string]: string; };
    connectTimeout?: number;
    options?: ChannelOptions;
}

export interface Config {
    $schema?: string;
    namespace?: string;
    importRoot?: string;
    protoPaths: string[];
    /** @deprecated use `protoPaths` instead. */
    protoDirs?: string[];
    protoOptions?: ProtoOptions,
    tsconfig?: string;
    /** @deprecated use `App.entry` instead. */
    entry: string;
    apps: App[];
}

/**
 * This type represents an interface that supports lifecycle events on the gRPC app. If a service
 * implements this interface, when the app starts and the service is loaded (or reloaded), the
 * `init()` method will be automatically called, we can add some async logic inside it, for example,
 * establishing database connection, which is normally not possible in the default `constructor()`
 * method since it doesn't support asynchronous codes. And when the app is about to stop, or the
 * service is about to be reloaded, the `destroy()` method will be called, which gives the ability
 * to clean up and release resource.
 */
export interface LifecycleSupportInterface {
    init(): Promise<void>;
    destroy(): Promise<void>;
}

/**
 * This type represents a struct of message that contains a `route` key that can be used in the 
 * internal client load balancer. When the client sending a request which implements this interface,
 * the program will automatically route the traffic to a certain server evaluated by the `route` key,
 * which can be set in the following forms:
 * 
 * - a URI that matches the ones that set in the config file;
 * - an app's name that matches the ones that set in the config file;
 * - if none of the above matches, use the hash algorithm on the `route` value;
 * - if `route` value is not set, then the default round-robin algorithm is used for routing.
 */
export interface RoutableMessageStruct {
    route: string;
}

export class RpcApp {
    name: string;
    private config: Config | null = null;
    private oldConfig: Config | null = null;
    private pkgDef: GrpcObject | null = null;
    private server: Server | null = null;
    private ctorsMap = new Map<string, {
        classCtor: ServiceClass;
        protoCtor: ServiceClientConstructor;
    }>();
    private instanceMap = new Map<string, any>();
    private sslOptions: { ca: Buffer, cert: Buffer, key: Buffer; } | null = null;
    private remoteServices = new Map<string, {
        instances: {
            app: string;
            uri: string;
            client: ServiceClient<object>;
        }[];
        counter: number;
    }>;

    private guest: Guest | null = null;
    private isProcessKeeper = false;

    private _onReload: (() => void) | null = null;
    private _onStop: (() => void) | null = null;

    static async loadConfig() {
        const defaultFile = absPath("ngrpc.json");
        const localFile = absPath("ngrpc.local.json");
        let fileContent: string | undefined;

        if (await exists(localFile)) {
            fileContent = await fs.readFile(localFile, "utf8");
        } else if (await exists(defaultFile)) {
            fileContent = await fs.readFile(defaultFile, "utf8");
        } else {
            throw new Error(`unable to load config file: ${defaultFile}`);
        }

        const conf: Config = JSON.parse(fileContent as string);

        if (conf.entry && conf.apps?.length) {
            conf.apps.forEach(app => {
                app.entry ||= conf.entry;
            });
        }

        if (conf.protoDirs && !conf.protoPaths) {
            conf.protoPaths = conf.protoDirs;
            delete conf.protoDirs;
        }

        if (conf.protoOptions) {
            // In the config file, we set the following properties in string format, whereas they
            // needs to be the corresponding constructor, so we convert them before using them.
            if ((conf.protoOptions.longs as any) === "String") {
                conf.protoOptions.longs = String;
            } else if ((conf.protoOptions.longs as any) === "Number") {
                conf.protoOptions.longs = Number;
            }

            if ((conf.protoOptions.enums as any) === "String") {
                conf.protoOptions.enums = String;
            }

            if ((conf.protoOptions.bytes as any) === "Array") {
                conf.protoOptions.bytes = Array;
            } else if ((conf.protoOptions.bytes as any) === "String") {
                conf.protoOptions.bytes = String;
            }
        }

        return conf;
    }

    static async loadConfigForPM2() {
        const conf = await this.loadConfig();
        const resolveEntry = (entry = "") => {
            entry ||= path.join(__dirname, "cli");
            const ext = path.extname(entry);

            if (ext === ".ts") {
                entry += entry.slice(0, -ext.length) + ".js";
            } else if (!ext) {
                entry += ".js";
            } else if (ext !== ".js") {
                throw new Error(`entry file '${entry}' is not a JavaScript file`);
            }

            return entry;
        };

        return {
            apps: conf.apps.filter(app => app.serve).map(app => {
                const _app: {
                    name: string;
                    script: string;
                    args: string,
                    env?: { [name: string]: string; };
                    log_file?: string;
                    out_file?: string;
                    err_file?: string;
                } = {
                    name: app.name,
                    script: app.entry ? resolveEntry(app.entry) : path.resolve(__dirname, "cli.js"),
                    args: [app.name]
                        .map(arg => arg.includes(" ") ? `"${arg}"` : arg)
                        .join(" "),
                    env: app.env || {},
                };

                if (app.stdout && app.stderr) {
                    if (app.stdout === app.stderr) {
                        _app["log_file"] = app.stdout;
                    } else {
                        _app["out_file"] = app.stdout;
                        _app["err_file"] = app.stderr;
                    }
                } else if (app.stdout) {
                    _app["log_file"] = app.stdout;
                }

                return _app;
            })
        };
    }

    protected async loadProtoFiles(dirs: string[], options?: ProtoOptions) {
        // recursively scan for `.proto` files
        const filenames: string[] = await (async function scan(dirs: string[]) {
            const filenames: string[] = [];

            for (const dir of dirs) {
                const files = await fs.readdir(dir);
                const subDirs: string[] = [];

                for (const file of files) {
                    const filename = path.join(dir, file);
                    const stat = await fs.stat(filename);

                    if (stat.isFile() && file.endsWith(".proto")) {
                        filenames.push(filename);
                    } else if (stat.isDirectory()) {
                        subDirs.push(filename);
                    }
                }

                if (subDirs.length) {
                    filenames.push(...await scan(subDirs));
                }
            }

            return filenames;
        })(dirs);

        // `load()` method is currently buggy, so we use `loadSync()` instead.
        const source = loadSync(filenames, options);
        this.pkgDef = loadPackageDefinition(source);
    }

    protected async loadClassFiles(apps: App[], importRoot = "") {
        for (const app of apps) {
            if (!app.services?.length) {
                continue;
            }

            for (const serviceName of app.services) {
                let filename = serviceName.split(".").join(path.sep);

                if (importRoot) {
                    filename = path.join(importRoot, filename);
                }

                if (!filename.startsWith(process.cwd())) {
                    filename = path.join(process.cwd(), filename);
                }

                const exports = await import(filename);
                let classCtor: ServiceClass;

                // The service class file must be implemented as a module that has a default export
                // which is the very service class itself.
                if (typeof exports.default === "function") {
                    classCtor = exports.default;
                } else {
                    throw new Error(`service '${serviceName}' is not correctly implemented`);
                }

                const protoServiceName = classCtor[sServiceName];
                const protoCtor: ServiceClientConstructor = get(this.pkgDef, protoServiceName);

                if (!protoCtor) {
                    throw new Error(`service '${serviceName}' is not correctly declared`);
                }


                if (!classCtor[sServiceName]) {
                    throw new Error(`service '${serviceName}' must be decorated by @service()`);
                } else {
                    this.ctorsMap.set(serviceName, { classCtor, protoCtor });
                }
            }
        }
    }

    protected getAddress(app: App, url: URL): { address: string, useSSL: boolean; } {
        const { protocol, hostname, port } = url;
        let address = hostname;
        let useSSL = !!(app.ca && app.cert && app.key);

        if (protocol === "grpcs:" || protocol === "https:") {
            if (!app.ca) {
                throw new Error(`missing 'ca' config for app [${app.name}]`);
            } else if (!app.cert) {
                throw new Error(`missing 'cert' config for app [${app.name}]`);
            } else if (!app.key) {
                throw new Error(`missing 'key' config for app [${app.name}]`);
            } else if (!port) {
                address += ":443";
            } else {
                address += ":" + port;
            }

            useSSL = true;
        } else if ((protocol === "grpc:" || protocol === "http:") && !port) {
            address += ":80";
        } else if (port) {
            address += ":" + port;
        }

        return { address, useSSL };
    }

    protected async initServer(app: App, reload = false) {
        const url = new URL(app.uri);

        if (url.protocol === "xds:") {
            throw new Error(`app [${app.name}] cannot be served since it uses 'xds:' protocol`);
        }

        const { address, useSSL } = this.getAddress(app, url);
        let newServer = !this.server;
        let ca: Buffer;
        let cert: Buffer;
        let key: Buffer;

        if (reload) {
            if (this.sslOptions) {
                if (!useSSL) { // SSL to non-SSL
                    this.sslOptions = null;
                    newServer = true;
                } else {
                    ca = await fs.readFile(app.ca as string);
                    cert = await fs.readFile(app.cert as string);
                    key = await fs.readFile(app.key as string);

                    if (Buffer.compare(ca, this.sslOptions.ca) ||
                        Buffer.compare(cert, this.sslOptions.cert) ||
                        Buffer.compare(key, this.sslOptions.key)
                    ) {
                        // SSL credentials changed
                        this.sslOptions = { ca, cert, key };
                        newServer = true;
                    }
                }
            } else if (useSSL) { // non-SSL to SSL
                ca = await fs.readFile(app.ca as string);
                cert = await fs.readFile(app.cert as string);
                key = await fs.readFile(app.key as string);
                this.sslOptions = { ca, cert, key };
                newServer = true;
            }

            if (this.oldConfig) {
                const oldApp = this.oldConfig.apps.find(_app => _app.name === app.name);

                if (!oldApp || oldApp.uri !== app.uri || !isEqual(oldApp.options, app.options)) {
                    // server configurations changed
                    newServer = true;
                }
            }
        } else if (useSSL) {
            ca = await fs.readFile(app.ca as string);
            cert = await fs.readFile(app.cert as string);
            key = await fs.readFile(app.key as string);
            this.sslOptions = { ca, cert, key };
        }

        if (newServer) {
            this.server?.forceShutdown();
            this.server = new Server(app.options);
        }

        for (const serviceName of (app.services as string[])) {
            let ins: any;

            const ctors = this.ctorsMap.get(serviceName);

            if (!ctors) {
                throw new Error(`service '${serviceName}' is not correctly declared`);
            }

            const { classCtor, protoCtor } = ctors;

            if (typeof classCtor.getInstance === "function") {
                // Support explicit singleton designed, in case the service should used in another
                // place without RPC tunneling.
                ins = classCtor.getInstance();
            } else {
                ins = new classCtor();
            }

            if (typeof ins.init === "function") {
                await ins.init();
            }

            serve(this.server as Server, protoCtor, ins);
            this.instanceMap.set(serviceName, ins);
        }

        if (newServer) {
            await new Promise<void>(async (resolve, reject) => {
                let cred: ServerCredentials;

                // Create different knd of server credentials according to whether the certificates
                // are set.
                if (ca && cert && key) {
                    cred = ServerCredentials.createSsl(ca, [{
                        private_key: key,
                        cert_chain: cert,
                    }], true);
                } else {
                    cred = ServerCredentials.createInsecure();
                }

                (this.server as Server).bindAsync(address, cred, (err) => {
                    if (err) {
                        reject(err);
                    } else {
                        (this.server as Server).start();
                        console.info(timed`app [${app.name}] started (pid: ${process.pid})`);
                        resolve();
                    }
                });
            });
        }
    }

    protected async initClients() {
        const conf = this.config as Config;
        const cas = new Map<string, Buffer>();
        const certs = new Map<string, Buffer>();
        const keys = new Map<string, Buffer>();

        // Preload `cert`s and `key`s so in the "connection" phase, there would be no latency for
        // loading resources asynchronously.
        for (const app of conf.apps) {
            if (app.ca) {
                const filename = absPath(app.ca);
                const ca = await fs.readFile(filename);
                cas.set(filename, ca);
            }

            if (app.cert) {
                const filename = absPath(app.cert);
                const cert = await fs.readFile(filename);
                certs.set(filename, cert);
            }

            if (app.key) {
                const filename = absPath(app.key);
                const key = await fs.readFile(filename);
                keys.set(filename, key);
            }
        }

        // IMPORTANT: keep the remaining code synchronous.

        // Close old clients and reset registry.
        this.remoteServices.forEach(({ instances }) => {
            instances.forEach(item => {
                item.client.close();
            });
        });
        this.remoteServices = new Map();

        // Set global namespace.
        const nsp = conf.namespace || "services";
        set(global, nsp, createChainingProxy(nsp, this));

        // Reorganize client-side configuration based on the service name since the gRPC clients are
        // created based on the service.
        const serviceApps = conf.apps.reduce((registry, app) => {
            for (const serviceName of (app.services as string[])) {
                const ctors = this.ctorsMap.get(serviceName);

                if (ctors) {
                    const { protoCtor } = ctors;
                    const apps = registry.get(serviceName);

                    if (apps) {
                        apps.push({ ...app, protoCtor });
                    } else {
                        registry.set(serviceName, [{ ...app, protoCtor }]);
                    }
                }
            }

            return registry;
        }, new Map<string, (App & { protoCtor: ServiceClientConstructor; })[]>());

        const getAddress = (app: App) => {
            const url = new URL(app.uri);

            if (url.protocol === "xds:") {
                return app.uri;
            } else {
                const { address } = this.getAddress(app, url);
                return address;
            }
        };
        const getConnectConfig = (app: App) => {
            const address = getAddress(app);
            let cred: ChannelCredentials;

            if (app.ca && app.cert && app.key) {
                const ca = cas.get(absPath(app.ca));
                const cert = certs.get(absPath(app.cert));
                const key = keys.get(absPath(app.key));
                cred = credentials.createSsl(ca, key, cert);
            } else {
                cred = credentials.createInsecure();
            }

            return { address, credentials: cred };
        };

        for (const [serviceName, apps] of serviceApps) {
            for (const app of apps) {
                const { protoCtor, options, connectTimeout } = app;
                const { address, credentials: cred } = getConnectConfig(app);
                const client = connect(protoCtor, address, cred, {
                    ...(options ?? null),
                    connectTimeout: connectTimeout || 5_000,
                });

                const remoteService = this.remoteServices.get(serviceName);

                if (remoteService) {
                    remoteService.instances.push({ app: app.name, uri: app.uri, client });
                } else {
                    this.remoteServices.set(serviceName, {
                        instances: [{ app: app.name, uri: app.uri, client }],
                        counter: 0,
                    });
                }
            }
        }
    }

    getServiceClient(serviceName: string, route: string = ""): ServiceClient<object> {
        const remoteService = this.remoteServices.get(serviceName);

        if (!remoteService) {
            throw new Error(`service ${serviceName} is not registered`);
        }

        const instances = remoteService.instances.filter(item => {
            const state = item.client.getChannel().getConnectivityState(false);
            return state != connectivityState.SHUTDOWN;
        });

        if (!instances.length) {
            throw new Error(`service ${serviceName} is not available`);
            // return null as unknown as ServiceClient<object>;
        }

        let client: ServiceClient<object>;

        if (route) {
            // First, try to match the route directly against the services' uris, if match any,
            // return it respectively.
            const item = instances.find(item => item.app === route || item.uri === route);

            if (item) {
                client = item.client;
            } else {
                // Then, try to use the hash algorithm to retrieve a remote instance.
                const id = hash(route);
                const idx = id % instances.length;
                client = instances[idx].client;
            }
        } else {
            // Use round-robin algorithm by default.
            const idx = remoteService.counter % instances.length;
            client = instances[idx].client;
        }

        return client;
    }

    protected async _start(name: string | null = "", config: Config) {
        this.config = config;
        let xdsApp: App | undefined;
        let app: App | undefined;

        if (!xdsEnabled &&
            (xdsApp = config.apps?.find(item => !!item.serve && item.uri?.startsWith("xds:")))
        ) {
            try {
                const _module = require("@grpc/grpc-js-xds");
                _module.register();
                xdsEnabled = true;
            } catch (err) {
                if (err["code"] === "MODULE_NOT_FOUND") {
                    throw new Error(`'xds:' protocol is used in app [${xdsApp.name}]`
                        + `, but package '@grpc/grpc-js-xds' is not installed`);
                }
            }
        }

        if (name) {
            this.name = name;
            app = (this.config as Config).apps.find(app => app.name === name);

            if (!app) {
                throw new Error(`app [${name}] is not configured`);
            }
        }

        await this.loadProtoFiles(config.protoPaths, config.protoOptions);
        await this.loadClassFiles(config.apps, process.env["IMPORT_ROOT"] || config.importRoot);

        if (app?.serve && app?.services?.length) {
            await this.initServer(app);
        }

        await this.initClients();
        this.guest = new Guest(app || ({ name: "", uri: "", }), {
            onStopCommand: (msgId) => {
                this._stop(msgId, true);
            },
            onReloadCommand: (msgId) => {
                this._reload(msgId);
            },
        });
        await this.guest.join();
    }

    /**
     * Initiates and starts an app.
     * 
     * @param name The app's name that should be started as a server. If not provided, the app only
     *  connects to other servers but not serves as one.
     */
    static async boot(name: string | null = "",) {
        const ins = new RpcApp();
        const config = await this.loadConfig();

        await ins._start(name, config);
        return ins;
    }

    /** Stops the app programmatically. */
    async stop() {
        return await this._stop("", true);
    }

    protected async _stop(msgId = "", graceful = false) {
        for (const [_, ins] of this.instanceMap) {
            if (typeof ins.destroy === "function") {
                await (ins as LifecycleSupportInterface).destroy();
            }
        }

        this.remoteServices.forEach(({ instances }) => {
            instances.forEach(item => {
                item.client.close();
            });
        });

        this.server?.forceShutdown();
        this._onStop?.();

        let msg: string;

        if (this.name) {
            msg = `app [${this.name}] stopped`;
            console.info(timed`${msg}`);
        } else {
            msg = "app (anonymous) stopped";
        }

        if (this.guest && graceful) {
            this.guest.leave(msg, msgId);

            if (this.name) {
                console.info(timed`app [${this.name}] has left the group`);
            }
        }

        if (this.isProcessKeeper) {
            process.exit(0);
        }
    }

    /** Reloads the app programmatically. */
    async reload() {
        return await this._reload();
    }

    protected async _reload(msgId = "") {
        this.oldConfig = this.config;
        this.config = await RpcApp.loadConfig();

        await this.loadProtoFiles(this.config.protoPaths, this.config.protoOptions);

        const app = this.config.apps.find(app => app.name === this.name);

        if (this.server) {
            // Unserve old service instance and possibly call the `destroy()` lifecycle method.
            for (const [_, ins] of this.instanceMap) {
                if (typeof ins.destroy === "function") {
                    await (ins as LifecycleSupportInterface).destroy();
                }
            }

            const importRoot = process.env["IMPORT_ROOT"] || this.config.importRoot || "";

            // Remove cached files and their dependencies so, when reloading, they could be
            // reimported and use any changes inside them.
            const filenames = [...this.ctorsMap].map(([serviceName, { protoCtor }]) => {
                unserve(this.server as Server, protoCtor);

                const basename = serviceName.split(".").join(path.sep);

                if (isTsNode) {
                    return path.join(process.cwd(), importRoot, basename + ".ts");
                } else {
                    return path.join(process.cwd(), importRoot, basename + ".js");
                }
            });
            const dependencies = findDependencies(filenames);

            [...filenames, ...dependencies].forEach(filename => {
                delete require.cache[filename];
            });

            // reset the server registry
            this.instanceMap = new Map();

            // reload class files
            await this.loadClassFiles(this.config.apps, importRoot);
        }

        if (app && app.serve && app.services?.length) {
            await this.initServer(app, true);
        } else if (this.server) { // The app has been removed from the config or no longer serve.
            this.server.forceShutdown();
            this.server = null;
        }

        await this.initClients();

        if (msgId) {
            // If `msgId` is provided, that means the stop event is issued by a guest app, for
            // example, the CLI tool, in this case, we need to send feedback to acknowledge the
            // sender that the process has finished.
            let msg: string;

            if (this.name) {
                msg = `app [${this.name}] hot-reloaded`;
                console.info(timed`${msg}`);
            } else {
                msg = "app (anonymous) hot-reloaded";
            }

            this.guest?.send({ cmd: "reply", msgId: msgId, text: msg });
        }

        this._onReload?.();
    }

    /** Registers a callback to run after the app is reloaded. */
    onReload(callback: () => void) {
        this._onReload = callback;
    }

    /** Registers a callback to run after the app is stopped. */
    onStop(callback: () => void) {
        this._onStop = callback;
    }

    /**
     * Listens for system signals to exit the program.
     * 
     * This method calls the `stop()` method internally, if we don't use this method, we need to
     * call the `stop()` method explicitly when the program is going to terminate.
     */
    waitForExit() {
        this.isProcessKeeper = true;
        const close = () => {
            this._stop("", true);
        };

        process.on("SIGINT", close).on("SIGTERM", close)
            .on("message", (msg) => {
                if (msg === "shutdown") { // for PM2 in Windows
                    close();
                }
            });
    }

    /**
     * Runs a snippet inside the apps context.
     * 
     * This function is for temporary scripting usage, it starts a runs pure-clients app so we
     * can use the services as we normally do in our program, and after the main `fn` function runs,
     * the app is automatically stopped.
     * 
     * @param fn The function to be run.
     */
    static async runSnippet(fn: () => void | Promise<void>) {
        let app: RpcApp | undefined;

        try {
            app = new RpcApp();
            const config = await this.loadConfig();

            await app._start(null, config);
            await fn();
            app.stop();
        } catch (err) {
            app?.stop();
            throw err;
        }
    }
}

const ngrpc = RpcApp;
export default ngrpc;
