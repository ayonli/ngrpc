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
import { serve, unserve, connect, ServiceClient } from "@hyurl/grpc-async";
import get = require("lodash/get");
import set = require("lodash/set");
import isEqual = require("lodash/isEqual");
import hash = require("string-hash");
import * as net from "net";
import isSocketResetError = require("is-socket-reset-error");
import {
    CpuUsage,
    absPath,
    ensureDir,
    exists,
    getCpuUsage,
    isTsNode,
    sServiceName,
    spawnProcess
} from "./util";
import { findDependencies } from "require-chain";
import humanizeDuration = require("humanize-duration");
import { createChainingProxy } from "./nsp";

export type { ServiceClient };

let xdsEnabled = false;

export type ServiceClass = (new () => any) & {
    getInstance?(): any;
    [sServiceName]: string;
};

/** These type represents the structures and properties set in the config file. */
export type Config = {
    $schema?: string;
    namespace?: string;
    entry?: string;
    importRoot?: string;
    /** @deprecated use `protoPaths` instead. */
    protoDirs?: string[];
    protoPaths: string[];
    protoOptions?: ProtoOptions,
    apps: {
        name: string;
        uri: string;
        serve?: boolean;
        services: string[];
        ca?: string;
        cert?: string;
        key?: string;
        connectTimeout?: number;
        options?: ChannelOptions;
        stdout?: string;
        stderr?: string;
        entry?: string;
        env?: { [name: string]: string; };
    }[];
};

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
    protected name: string;
    protected config: Config | null = null;
    protected oldConfig: Config | null = null;
    protected pkgDef: GrpcObject | null = null;
    protected server: Server | null = null;
    protected ctorsMap = new Map<string, {
        classCtor: ServiceClass;
        protoCtor: ServiceClientConstructor;
    }>();
    protected instanceMap = new Map<string, any>();
    protected sslOptions: { ca: Buffer, cert: Buffer, key: Buffer; } | null = null;
    protected serverOptions: ChannelOptions | null = null;
    protected remoteServices = new Map<string, {
        instances: {
            app: string;
            uri: string;
            client: ServiceClient<object>;
        }[];
        counter: number;
    }>;

    // The Host-Guest model is a mechanism used to hold communication between all apps running on
    // the same machine.
    //
    // When a app starts, regarding serves as a server or just pure clients, it joins the group and
    // tries to gain the hostship. If succeeds, it becomes the host app and others become guests.
    // The host app starts a guest client that connects to itself as well. Through the host app,
    // guests can talk to each other.
    //
    // This mechanism is primarily used for the CLI tool sending control commands to the apps. When
    // the CLI tool starts, it becomes one of the member of the group (it starts a pure clients app),
    // and use this communication channel to send commands like `reload` and `stop` to other apps.
    protected host: net.Server | null = null;
    protected hostClients: { socket: net.Socket, stopped: boolean; app?: string; }[] = [];
    protected hostCallbacks = new Map<string, (result: any) => void>();
    protected hostName: string | undefined = void 0;
    protected hostStopped = false;
    protected guest: net.Socket | null = null;
    protected canTryHost = false;

    protected isStopped = false;
    protected _onReload: (() => void) | null = null;
    protected _onStop: (() => void) | null = null;

    private cpuUsage: CpuUsage | null = null;

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
        const defaultEntry = resolveEntry(conf.entry);

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
                    script: app.entry ? resolveEntry(app.entry) : defaultEntry,
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

    protected async loadClassFiles(apps: Config["apps"], importRoot = "") {
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

    protected getAddress(app: Config["apps"][0], url: URL): { address: string, useSSL: boolean; } {
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

    protected async initServer(app: Config["apps"][0], reload = false) {
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

        for (const serviceName of app.services) {
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
                        console.info(`app [${app.name}] started at '${app.uri}'`);
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
            for (const serviceName of app.services) {
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
        }, new Map<string, (Config["apps"][0] & { protoCtor: ServiceClientConstructor; })[]>());

        const getAddress = (app: Config["apps"][0]) => {
            const url = new URL(app.uri);

            if (url.protocol === "xds:") {
                return app.uri;
            } else {
                const { address } = this.getAddress(app, url);
                return address;
            }
        };
        const getConnectConfig = (app: Config["apps"][0]) => {
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

    protected async _start(name: string | null = "", tryHost = false) {
        const config = this.config as Config;
        let xdsApp: Config["apps"][0] | undefined;
        let app: Config["apps"][0] | undefined;

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
        await this.loadClassFiles(config.apps, config.importRoot);

        if (app?.serve && app?.services?.length) {
            await this.initServer(app);
        }

        await this.initClients();

        if (tryHost) {
            this.canTryHost = true;
            await this.tryHosting();
        } else {
            await this.tryJoining();
        }
    }

    /**
     * Initiates and starts an app.
     * 
     * @param name The app's name that should be started as a server. If not provided, the app only
     *  connects to other servers but not serves as one.
     */
    static async boot(name: string | null = "",) {
        const ins = new RpcApp();

        ins.config = await this.loadConfig();

        // When starting, if no `app` is provided and no `require.main` is presented, that means the
        // the is running in the Node.js REPL and we're trying only to connect to services, in this
        // case, we don't need to try gaining the hostship.
        await ins._start(name, !!(name || require.main));

        return ins;
    }

    /** Stops the app programmatically. */
    async stop() {
        return await this._stop();
    }

    protected async _stop(msgId = "") {
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
        this.isStopped = true;

        const closeGuestSocket = () => {
            if (msgId) {
                // If `msgId` is provided, that means the stop event is issued by a guest app, for
                // example, the CLI tool, in this case, we need to send feedback to acknowledge the
                // sender that the process has finished.
                let result: string;

                if (this.name) {
                    result = `app [${this.name}] stopped`;
                    console.info(result);
                } else {
                    result = "app (clients) stopped";
                }

                this.guest?.end(JSON.stringify({
                    cmd: "reply",
                    msgId: msgId,
                    result
                }));
            } else {
                this.guest?.end();
            }
        };

        if (this.host) {
            this.hostClients.forEach(client => {
                client.socket.write(JSON.stringify({
                    cmd: "goodbye",
                    app: this.name,
                }));
            });

            closeGuestSocket();
            await new Promise<void>((resolve, reject) => {
                this.host?.close((err) => {
                    if (err) {
                        reject(err);
                    } else {
                        this._onStop?.();
                        resolve();
                    }
                });
            });
        } else {
            this.guest?.write(JSON.stringify({
                cmd: "goodbye",
                app: this.name,
            }) + "\n");
            closeGuestSocket();
            this._onStop?.();
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

            // Remove cached files and their dependencies so, when reloading, they could be
            // reimported and use any changes inside them.
            const filenames = [...this.ctorsMap].map(([serviceName, { protoCtor }]) => {
                unserve(this.server as Server, protoCtor);

                const basename = serviceName.split(".").join(path.sep);

                if (isTsNode) {
                    return path.join(process.cwd(), basename + ".ts");
                } else {
                    return path.join(process.cwd(), basename + ".js");
                }
            });
            const dependencies = findDependencies(filenames);

            [...filenames, ...dependencies].forEach(filename => {
                delete require.cache[filename];
            });

            // reset the server registry
            this.instanceMap = new Map();

            // reload class files
            await this.loadClassFiles(this.config.apps, this.config.importRoot);
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
            let result: string;

            if (this.name) {
                result = `app [${this.name}] reloaded`;
                console.info(result);
            } else {
                result = `app (clients) reloaded`;
            }

            this.guest?.write(JSON.stringify({ cmd: "reply", msgId: msgId, result }) + "\n");
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
     * Try to make the current app as the host app for communications.
     */
    protected async tryHosting() {
        const { success, filename, sockPath } = await this.tryJoining();

        if (success) {
            return;
        } else {
            await ensureDir(path.dirname(filename));
        }

        await new Promise<void>((resolve, reject) => {
            const host = net.createServer(client => {
                const conf = this.config as Config;

                client.on("data", (buf) => {
                    RpcApp.processSocketMessage(buf, (err, msg: {
                        cmd: "handshake" | "goodbye" | "reload" | "stop" | "list" | "reply",
                        app?: string,
                        msgId?: string;
                    }) => {
                        if (err) {
                            client.destroy(err);
                            return;
                        }

                        if (msg.cmd === "handshake") {
                            // After a guest establish the socket connection, it sends a `handshake`
                            // command indicates a signing-in, we then store the client in the
                            // `hostClients` property for broadcast purposes.

                            if (msg.app) {
                                if (!conf.apps.some(app => app.name === msg.app)) {
                                    client.destroy(new Error(`invalid app name '${msg.app}'`));
                                } else if (this.hostClients.some(item => item.app === msg.app)) {
                                    client.destroy(new Error(`app [${msg}] is already running`));
                                } else {
                                    this.hostClients.push({
                                        socket: client,
                                        stopped: false,
                                        app: msg.app,
                                    });
                                }
                            } else {
                                this.hostClients.push({ socket: client, stopped: false });
                            }

                            client.write(JSON.stringify({
                                cmd: "handshake",
                                app: this.name,
                            }) + "\n");
                        } else if (msg.cmd === "goodbye") {
                            this.hostClients.forEach(_client => {
                                if (_client.socket === client) {
                                    _client.stopped = true;
                                }
                            });
                        } else if (msg.cmd === "reload" || msg.cmd == "stop") {
                            // When the host app receives a control command, it distribute the
                            // command to the target app or all apps if the app is not specified.

                            if (msg.app) {
                                const _client = this.hostClients.find(item => item.app === msg.app);

                                if (_client) {
                                    const msgId = Math.random().toString(16).slice(2);

                                    _client.socket.write(JSON.stringify({
                                        cmd: msg.cmd,
                                        msgId,
                                    }) + "\n");
                                    this.hostCallbacks.set(msgId, (reply) => {
                                        client.end(JSON.stringify(reply));
                                    });
                                } else {
                                    client.destroy(new Error(`app [${msg.app}] is not running`));
                                }
                            } else {
                                const clients = [...this.hostClients];
                                let count = 0;

                                clients.forEach(_client => {
                                    const msgId = Math.random().toString(16).slice(2);

                                    if (_client.stopped) {
                                        count++;
                                    } else {
                                        _client.socket.write(JSON.stringify({
                                            cmd: msg.cmd,
                                            msgId,
                                        }) + "\n");
                                        this.hostCallbacks.set(msgId, (reply) => {
                                            client.write(JSON.stringify(reply) + "\n");

                                            if (++count === clients.length) {
                                                client.end();
                                            }
                                        });
                                    }
                                });
                            }
                        } else if (msg.cmd === "list") {
                            // When the host app receives a `list` command, we list all the clients
                            // with names and collect some information about them.
                            const clients = this.hostClients.filter(item => {
                                return !!item.app && !item.stopped;
                            });
                            type Stat = {
                                pid: number;
                                uptime: number;
                                memory: number;
                                cpu: number;
                            };
                            const stats = new Map<string, Stat>();

                            clients.forEach(_client => {
                                const msgId = Math.random().toString(16).slice(2);

                                _client.socket.write(JSON.stringify({
                                    cmd: "stat",
                                    msgId,
                                }) + "\n");
                                this.hostCallbacks.set(msgId, (reply: {
                                    result: Stat;
                                }) => {
                                    stats.set(_client.app as string, reply.result);

                                    if (stats.size === clients.length) {
                                        const list = clients.map(item => {
                                            const appName = item.app as string;
                                            const app = conf.apps
                                                .find(item => item.name === appName);
                                            const stat = stats.get(appName);

                                            return {
                                                app: appName,
                                                uri: app?.uri,
                                                ...stat
                                            };
                                        });

                                        client.end(JSON.stringify({ result: list }));
                                    }
                                });
                            });
                        } else if (msg.cmd === "reply") {
                            // When a guest app finishes a control command, it send feedback via the
                            // `reply` command, we use the `msgId` to retrieve the callback, run it
                            // and remove it.

                            if (msg.msgId) {
                                const callback = this.hostCallbacks.get(msg.msgId);

                                if (callback) {
                                    callback(msg);
                                    this.hostCallbacks.delete(msg.msgId);
                                }
                            }
                        } else {
                            client.destroy(new Error(`invalid message: ${JSON.stringify(msg)}`));
                        }
                    });
                }).on("close", () => {
                    const _client = this.hostClients.find(item => item.socket === client);

                    if (_client) {
                        // Remove the client from the list if it has been stopped.
                        this.hostClients = this.hostClients.filter(item => item !== _client);

                        // When the client is closed expectedly, it sends a `goodbye` command to the
                        // host server and the later marked it as `stopped` normally. Otherwise, the
                        // connection is closed due to program failure on the client-side, we can
                        // try to revive the client app.
                        if (_client.app &&
                            _client.app !== this.name &&
                            !_client.stopped &&
                            !this.hostStopped
                        ) {
                            const app = conf.apps.find(app => app.name === _client.app);

                            if (app) {
                                spawnProcess(app, conf.entry).catch(console.error);
                            }
                        }
                    }
                });
            }).once("error", err => {
                if (err["code"] === "EEXIST") { // gaining hostship failed
                    this.doTryJoining(sockPath, resolve, reject);
                } else {
                    reject(err);
                }
            });

            host.listen(sockPath, () => {
                this.host = host;

                // Connect to self so that we can use the same control logic.
                this.doTryJoining(sockPath, resolve, reject);

                if (this.name) {
                    console.info(`app [${this.name}] has become the host server`);
                } else {
                    console.info("this app has become the host server");
                }
            });
        });
    }

    protected async tryJoining(): Promise<{
        success: boolean;
        filename: string;
        sockPath: string;
    }> {
        const config = absPath("ngrpc.json");
        const ext = path.extname(config);
        const filename = config.slice(0, -ext.length) + ".sock";
        const sockPath = absPath(filename, true);

        if (await exists(filename)) {
            // If the socket file already exists, there either be the host app already exists or the
            // socket file is left there because an unclean shutdown of the previous host app. We
            // need to first try to connect to it, if not succeeded, delete it and try to gain the
            // hostship.
            try {
                await new Promise<void>((resolve, reject) => {
                    this.doTryJoining(sockPath, resolve, reject);
                });
                return { success: true, filename, sockPath };
            } catch {
                await fs.unlink(filename);
            }
        }

        return { success: false, filename, sockPath };
    }

    /**
     * Try to connect to the host server of app control.
     * 
     * @param sockPath 
     * @param resolve 
     * @param reject 
     */
    protected doTryJoining(sockPath: string, resolve: () => void, reject: (err: Error) => void) {
        const client = net.createConnection(sockPath, () => {
            client.write(JSON.stringify({ cmd: "handshake", app: this.name }) + "\n");
            this.guest = client;

            if (!this.canTryHost) {
                resolve();
            }
        });
        const retryHost = () => {
            const lastHostName = this.hostName;
            const hostStopped = this.hostStopped;

            // If the connection is closed and the client is not marked closed, the client now is
            // able to retry gaining the hostship.
            this.tryHosting().then(() => {
                // When the host server is closed by the expectedly, it sends a `goodbye`
                // command to the clients to acknowledge a clear shutdown, and the client
                // mark `hostStopped`, otherwise, the connection is closed unexpectedly due
                // to program failure, after the current app has gain the hostship, we can
                // try to revive the formal host app.
                if (lastHostName && !hostStopped) {
                    const conf = this.config as Config;
                    const app = conf.apps.find(app => app.name === lastHostName);

                    if (app) {
                        return spawnProcess(app, conf.entry);
                    }
                }
            }).catch(console.error);
        };

        client.on("data", (buf) => {
            RpcApp.processSocketMessage(buf, (err, msg: {
                cmd: "handshake" | "goodbye" | "reload" | "stop" | "stat";
                app?: string;
                msgId: string;
            }) => {
                if (err) {
                    return; // ignore invalid messages
                }

                if (msg.cmd === "handshake") {
                    this.hostName = msg.app;
                    this.hostStopped = false;

                    if (this.canTryHost) {
                        resolve();
                    }
                } else if (msg.cmd === "goodbye") {
                    this.hostStopped = true;

                    if (!this.host) {
                        client.end();
                    }
                } else if (msg.cmd === "reload") {
                    this._reload(msg.msgId).catch(console.error);
                } else if (msg.cmd === "stop") {
                    this._stop(msg.msgId).catch(console.error);
                } else if (msg.cmd === "stat") {
                    client.write(JSON.stringify({
                        cmd: "reply",
                        msgId: msg.msgId,
                        result: {
                            pid: process.pid,
                            uptime: process.uptime(),
                            memory: process.memoryUsage().rss,
                            cpu: (this.cpuUsage = getCpuUsage(this.cpuUsage)).percent,
                        }
                    }) + "\n");
                } else {
                    // ignore other messages
                }
            });
        }).on("end", () => {
            if (!this.host && !this.isStopped && this.canTryHost) {
                retryHost();
            }
        }).on("error", err => {
            if (isSocketResetError(err) && !this.host && !this.isStopped) {
                if (this.canTryHost) {
                    retryHost();
                }
            } else {
                reject(err);
            }
        });
    }

    private static processSocketMessage(buf: Buffer, handle: (err: Error | null, msg: any) => void) {
        const str = buf.toString();

        try {
            const chunks = str.split("\n");

            for (const chunk of chunks) {
                if (!chunk)
                    continue;

                try {
                    const msg = JSON.parse(chunk);
                    handle(null, msg);
                } catch {
                    handle(new Error("invalid message: " + str), null);
                }
            }
        } catch {
            handle(new Error("invalid message: " + str), null);
        }
    }

    /**
     * Sends control command to the apps. This function is mainly used in the CLI tool.
     * 
     * @param cmd 
     * @param app The app's name that should received the command. If not provided, the command is
     *  sent to all apps.
     */
    static async sendCommand(cmd: "reload" | "stop" | "list", app: string | null = "") {
        const config = absPath("ngrpc.json");
        const ext = path.extname(config);
        const sockFile = absPath(config.slice(0, -ext.length) + ".sock", true);
        type Stat = {
            app: string;
            uri: string;
            pid: number;
            uptime: number;
            memory: number;
            cpu: number;
        };

        const listApps = async (stats: Stat[]) => {
            const { apps } = await this.loadConfig();
            // const result = reply.result as Stat[];
            const list = apps.map(app => {
                const item = stats.find(item => item.app === app.name);

                if (item) {
                    return item;
                } else if (app.serve) {
                    return {
                        app: app.name,
                        uri: app.uri,
                        pid: -1,
                        uptime: -1,
                        memory: -1,
                        cpu: -1,
                    } satisfies Stat;
                } else {
                    return null as unknown as Stat;
                }
            }).filter(item => !!item);

            stats.forEach(item => {
                const _item = list.find(_item => _item.app === item.app);

                if (!_item) {
                    list.push(item);
                }
            });

            console.table(list.map(item => {
                return {
                    app: item.app,
                    uri: item.uri,
                    status: item.pid !== -1 ? "running" : "stopped",
                    pid: item.pid === -1 ? "N/A" : item.pid,
                    uptime: item.uptime === -1
                        ? "N/A"
                        : humanizeDuration(item.uptime * 1000, {
                            largest: 1,
                            round: true,
                        }),
                    memory: item.memory === -1
                        ? "N/A"
                        : `${Math.round(item.memory / 1024 / 1024 * 100) / 100} MB`,
                    cpu: item.cpu === -1 ? "N/A" : `${Math.round(item.cpu)}%`,
                };
            }));
        };

        await new Promise<void>((resolve, reject) => {
            const client = net.createConnection(sockFile, () => {
                client.write(JSON.stringify({ cmd, app }) + "\n");
            });

            client.on("data", (buf) => {
                this.processSocketMessage(buf, (err, reply: { result?: any; error?: string; }) => {
                    if (err) {
                        console.error(err);
                        return;
                    }

                    if (reply.error) {
                        console.error(reply.error);
                    } else if (cmd === "list") {
                        listApps(reply.result).catch(console.error);
                    } else {
                        console.info(reply.result ?? null);
                    }
                });
            }).on("end", resolve).once("error", (err) => {
                if (err["code"] === "ENOENT" || err["code"] === "ECONNREFUSED") {
                    if (cmd === "list") {
                        listApps([]).catch(console.error);
                    } else if (app) {
                        console.info(`app [${app}] is not running`);
                    } else {
                        console.info("no app is running");
                    }

                    resolve();
                } else {
                    reject(err);
                }
            });
        });
    }

    /**
     * Runs a snippet inside the apps context.
     * 
     * This function is for temporary scripting usage, it starts a temporary pure-clients app so we
     * can use the services as we normally do in our program, and after the main `fn` function runs,
     * the app is automatically stopped.
     * 
     * @param fn The function to be run.
     */
    static async runSnippet(fn: () => void | Promise<void>) {
        try {
            const app = new RpcApp();

            app.config = await this.loadConfig();

            await app._start();
            await fn();
            await app.stop();
        } catch (err) {
            console.error(err);
        }
    }
}

const ngrpc = RpcApp;
export default ngrpc;
