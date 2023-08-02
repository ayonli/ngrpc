import * as fs from "fs";
import * as path from "path";
import * as FRON from "fron";
import { Options as ProtoOptions, loadSync } from "@grpc/proto-loader";
import {
    ChannelCredentials,
    ChannelOptions,
    GrpcObject,
    Server,
    ServerCredentials,
    ServiceClientConstructor,
    credentials,
    loadPackageDefinition
} from "@grpc/grpc-js";
import { serve, unserve, connect, LoadBalancer, ConnectionManager, ServerConfig } from "@hyurl/grpc-async";
import get = require("lodash/get");
import isEqual = require("lodash/isEqual");
import hash = require("string-hash");
import * as net from "net";
import isSocketResetError = require("is-socket-reset-error");

export type Config = {
    package: string;
    protoDirs: string[];
    protoOptions: ProtoOptions,
    apps: {
        name: string;
        uri: string;
        serve?: boolean;
        services: string[];
        cert?: string;
        key?: string;
        connectTimeout?: number;
        options?: ChannelOptions;
    }[];
    sockFile?: string;
};

export interface BootLifecycleSupport {
    init(): Promise<void>;
    destroy(): Promise<void>;
}

export class BootApp {
    protected conf: Config = null;
    protected oldConf: Config = null;
    protected pkgDef: GrpcObject;
    protected server: Server;
    protected manager: ConnectionManager;
    protected serverName: string;
    protected serverRegistry = new Map<string, { ctor: ServiceClientConstructor, ins: any; }>();
    protected sslOptions: { cert: Buffer, key: Buffer; } = null;
    protected serverOptions: ChannelOptions = null;
    protected rootNsp: string;
    protected clientRegistry = new Map<string, Config["apps"]>();
    protected host: net.Server = null;
    protected hostClients = new Map<string, net.Socket>();
    protected hostCallbacks = new Map<string, (result: any) => void>();
    protected _onReload: () => void = null;
    protected _onStop: () => void = null;

    constructor(protected config: string = "") {
        this.conf = BootApp.loadConfig(config);
    }

    protected static loadConfig(config: string = "") {
        config = this.absPath(config || "boot.config.json");
        const fileContent = fs.readFileSync(config, "utf8");
        const conf: Config = FRON.parse(fileContent, config);

        if (conf.protoOptions) {
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

    protected async loadProtoFiles(dirs: string[], options: ProtoOptions, reload = false) {
        if (!reload && this.pkgDef)
            return;

        const filenames: string[] = await (async function scan(dirs: string[]) {
            const filenames: string[] = [];

            for (const dir of dirs) {
                const files = await fs.promises.readdir(dir);
                const subDirs: string[] = [];

                for (const file of files) {
                    const filename = path.join(dir, file);
                    const stat = await fs.promises.stat(filename);

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

        this.pkgDef = loadPackageDefinition(loadSync(filenames, options));
    }

    protected async initServer(appName: string, reload = false) {
        const app = this.conf.apps.find(app => app.name === appName);

        if (!app) {
            throw new Error(`gRPC app '${appName}' is not configured in the config file`);
        } else if (!app.serve) {
            throw new Error(`gRPC app '${appName}' is not intended to be served`);
        }

        const { protocol, hostname, port } = new URL(app.uri);
        let address = hostname;
        let newServer = !this.server;
        let useSSL = false;
        let cert: Buffer;
        let key: Buffer;

        if (protocol === "xds:") {
            throw new Error(`gRPC app '${app.name}' cannot be served since it uses 'xds:' protocol`);
        } else if (protocol === "grpcs:" || protocol === "https:") {
            if (!app.cert) {
                throw new Error(`Missing 'cert' config for app '${app.name}'`);
            } else if (!app.key) {
                throw new Error(`Missing 'key' config for app '${app.name}'`);
            } else if (!port) {
                address += ":443";
            }

            useSSL = true;
        } else if ((protocol === "grpc:" || protocol === "http:") && !port) {
            address += ":80";
        } else if (port) {
            address += ":" + port;
        }

        if (reload) {
            if (this.sslOptions) {
                if (!useSSL) { // SSL to non-SSL
                    this.sslOptions = null;
                    newServer = true;
                } else {
                    cert = await fs.promises.readFile(app.cert);
                    key = await fs.promises.readFile(app.key);

                    if (Buffer.compare(cert, this.sslOptions.cert) ||
                        Buffer.compare(key, this.sslOptions.key)
                    ) {
                        // SSL credentials changed
                        this.sslOptions = { cert, key };
                        newServer = true;
                    }
                }
            } else if (useSSL) { // non-SSL to SSL
                cert = await fs.promises.readFile(app.cert);
                key = await fs.promises.readFile(app.key);
                this.sslOptions = { cert, key };
                newServer = true;
            }

            if (this.oldConf) {
                const oldApp = this.oldConf.apps.find(app => app.name === appName);

                if (!oldApp || oldApp.uri !== app.uri || !isEqual(oldApp.options, app.options)) {
                    newServer = true;
                }
            }
        }

        if (newServer || !this.server) {
            this.server?.forceShutdown();
            this.server = new Server(app.options);
            this.serverName = app.name;
            await this.loadProtoFiles(this.conf.protoDirs, this.conf.protoOptions);
        }

        for (const serviceName of app.services) {
            const className = serviceName.split(".").slice(-1).toString();
            const filename = path.join(process.cwd(), serviceName.split(".").join(path.sep));

            if (reload) {
                delete require.cache[filename];
            }

            const exports = await import(filename);
            let ctor: (new () => any) & { getInstance(): any; };
            let ins: any;

            if (typeof exports.default === "function") {
                ctor = exports.default;
            } else if (typeof exports[className] === "function") {
                ctor = exports[className];
            } else {
                console.error(`service '${serviceName}' is not a valid service class`);
                continue;
            }

            if (typeof ctor.getInstance === "function") {
                ins = ctor.getInstance();
            } else {
                ins = new ctor();
            }

            if (reload && this.serverRegistry.has(serviceName)) {
                const target = this.serverRegistry.get(serviceName);

                if (typeof target.ins.destroy === "function") {
                    await target.ins.destroy();
                }

                unserve(this.server, target.ctor);
            }

            const protoCtor = get(this.pkgDef as object, serviceName);
            serve(this.server, protoCtor, ins);
            this.serverRegistry.set(serviceName, { ctor: protoCtor, ins });
        }

        if (newServer) {
            await new Promise<void>(async (resolve, reject) => {
                let cred: ServerCredentials;

                if (useSSL) {
                    cred = ServerCredentials.createSsl(cert, [{
                        private_key: key,
                        cert_chain: cert,
                    }], true);
                } else {
                    cred = ServerCredentials.createInsecure();
                }

                this.server.bindAsync(address, cred, (err) => {
                    this.server.start();

                    if (err) {
                        reject(err);
                    } else {
                        console.info(`gRPC server [${app.name}] started`);
                        resolve();
                    }
                });
            });
        }
    }

    protected async initClients(reload = false) {
        await this.loadProtoFiles(this.conf.protoDirs, this.conf.protoOptions);

        this.manager ||= new ConnectionManager();
        const rootNsp = this.conf.package;
        // @ts-ignore
        global[rootNsp] ||= this.manager.useChainingSyntax(rootNsp);

        const clientRegistry = this.conf.apps.reduce((registry, app) => {
            for (const serviceName of app.services) {
                const apps = registry.get(serviceName);

                if (apps) {
                    apps.push(app);
                } else {
                    registry.set(serviceName, [app]);
                }
            }

            return registry;
        }, new Map<string, Config["apps"]>());

        if (reload) {
            this.clientRegistry.forEach((_, serviceName) => {
                if (!clientRegistry.has(serviceName)) {
                    this.manager.deregister(serviceName, true);
                }
            });
        }


        for (const [serviceName, apps] of clientRegistry) {
            const getAddress = (app: Config["apps"][0]) => {
                const { protocol, hostname, port } = new URL(app.uri);
                let address = hostname;

                if (protocol === "xds:") {
                    address = app.uri;
                } else if (protocol === "grpcs:" || protocol === "https:") {
                    if (!app.cert) {
                        throw new Error(`Missing 'cert' config for app '${app.name}'`);
                    } else if (!app.key) {
                        throw new Error(`Missing 'key' config for app '${app.name}'`);
                    } else if (!port) {
                        address += ":443";
                    }
                } else if ((protocol === "grpc:" || protocol === "http:") && !port) {
                    address += ":80";
                } else if (port) {
                    address += ":" + port;
                }

                return address;
            };
            const getConnectConfig = async (app: Config["apps"][0]) => {
                const address = getAddress(app);
                let cred: ChannelCredentials;

                if (app.uri.startsWith("grpcs:")) {
                    const cert = await fs.promises.readFile(app.cert);
                    const key = await fs.promises.readFile(app.key);
                    cred = credentials.createSsl(cert, key, cert);
                } else {
                    cred = credentials.createInsecure();
                }

                return { address, credentials: cred };
            };

            if (reload) {
                this.manager.deregister(serviceName, true);
            }

            if (apps.length === 1) {
                const [app] = apps;
                const { address, credentials: cred } = await getConnectConfig(app);

                const client = connect(get(this.pkgDef as object, serviceName), address, cred, {
                    ...(app.options ?? null),
                    connectTimeout: app.connectTimeout || 120_000,
                });

                this.manager.register(client);
            } else {
                const servers: ServerConfig[] = [];

                for (const app of apps) {
                    const { address, credentials: cred } = await getConnectConfig(app);
                    servers.push({
                        address,
                        credentials: cred,
                        options: {
                            ...(app.options ?? null),
                            connectTimeout: app.connectTimeout || 120_000,
                        },
                    });
                }

                const balancer = new LoadBalancer(
                    get(this.pkgDef as object, serviceName),
                    servers,
                    (ctx) => {
                        const addresses: string[] = ctx.servers.map(item => item.address);

                        if (typeof ctx.params === "string") {
                            if (addresses.includes(ctx.params)) {
                                return ctx.params; // explicitly use a server instance
                            } else {
                                const app = apps.find(
                                    app => app.name === ctx.params || app.uri === ctx.params
                                );

                                if (app) {
                                    const address = getAddress(app);

                                    if (addresses.includes(address)) {
                                        return address;
                                    }
                                }

                                // route by hash
                                const id: number = hash(ctx.params);
                                return addresses[id % addresses.length];
                            }
                        } else if (typeof ctx.params === "number") {
                            return addresses[ctx.params % addresses.length];
                        } else if (typeof ctx.params === "object" && !Array.isArray(ctx.params)) {
                            // This algorithm guarantees the same param structure passed to the
                            // `getInstance()` returns the same service instance.
                            const id: number = hash(String(Object.keys(ctx.params ?? {}).sort()));
                            return addresses[id % addresses.length];
                        } else {
                            // use round-robin
                            return addresses[ctx.acc % addresses.length];
                        }
                    }
                );

                this.manager.register(balancer);
            }
        }

        this.clientRegistry = clientRegistry;
    }

    async start(appName = "") {
        if (appName) {
            await this.initServer(appName);
        }

        await this.initClients();

        for (const [, { ins }] of this.serverRegistry) {
            if (typeof ins.init === "function") {
                await ins.init();
            }
        }

        await this.tryHost();
    }

    async stop() {
        for (const [_, { ins }] of this.serverRegistry) {
            if (typeof ins.destroy === "function") {
                await ins.destroy();
            }
        }

        this.manager?.close();
        this.server?.forceShutdown();
    }

    onReload(callback: () => void) {
        this._onReload = callback;
    }

    onStop(callback: () => void) {
        this._onStop = callback;
    }

    protected async reload() {
        this.oldConf = this.conf;
        this.conf = BootApp.loadConfig(this.config);
        await this.loadProtoFiles(this.conf.protoDirs, this.conf.protoOptions, true);

        if (this.server) {
            await this.initServer(this.serverName, true);
        }

        await this.initClients(true);
    }

    protected async tryHost() {
        const _sockFile = BootApp.absPath(this.conf.sockFile || "boot.sock");

        await BootApp.ensureDir(path.dirname(_sockFile));
        // If the path exists, it's more likely caused by a previous 
        // server process closing unexpected, just remove it before ship
        // the new server.
        await BootApp.unlinkIfExists(_sockFile);

        const sockFile = BootApp.absPath(_sockFile, true);

        await new Promise<void>((resolve, reject) => {
            const host = net.createServer(client => {
                client.on("data", (buf) => {
                    try {
                        const msg = JSON.parse(buf.toString()) as {
                            cmd: string,
                            app?: string,
                            replyId?: string;
                        };

                        if (msg.cmd === "connect") {
                            if (msg.app) {
                                if (!this.conf.apps.some(app => app.name === msg.app)) {
                                    client.destroy(new Error(`Invalid app name ${msg.app}`));
                                } else {
                                    this.hostClients.set(msg.app, client);
                                }
                            }
                        } else if (msg.cmd === "reload") {
                            if (msg.app) {
                                const _client = this.hostClients.get(msg.app);

                                if (_client) {
                                    const replyId = Math.random().toString(16).slice(2);
                                    _client.write(JSON.stringify({ cmd: "reload", replyId }));
                                    this.hostCallbacks.set(replyId, (reply) => {
                                        client.end(reply);
                                    });
                                } else {
                                    client.destroy(new Error(`gRPC app ${msg.app} is not running`));
                                }
                            } else {
                                let count = 0;
                                let limit = this.hostClients.size;
                                this.hostClients.forEach(_client => {
                                    const replyId = Math.random().toString(16).slice(2);
                                    _client.write(JSON.stringify({ cmd: "reload", replyId }));
                                    this.hostCallbacks.set(replyId, (reply) => {
                                        client.write(JSON.stringify(reply));

                                        if (++count === limit) {
                                            client.end();
                                        }
                                    });
                                });
                            }
                        } else if (msg.cmd === "stop") {
                            if (msg.app) {
                                const _client = this.hostClients.get(msg.app);

                                if (_client) {
                                    const replyId = Math.random().toString(16).slice(2);
                                    _client.write(JSON.stringify({ cmd: "stop", replyId }));
                                    this.hostCallbacks.set(replyId, (reply) => {
                                        client.end(reply);
                                    });
                                } else {
                                    client.destroy(new Error(`gRPC app ${msg.app} is not running`));
                                }
                            } else {
                                let count = 0;
                                let limit = this.hostClients.size;
                                this.hostClients.forEach(_client => {
                                    const replyId = Math.random().toString(16).slice(2);
                                    _client.write(JSON.stringify({ cmd: "stop", replyId }));
                                    this.hostCallbacks.set(replyId, (reply) => {
                                        client.write(JSON.stringify(reply));

                                        if (++count === limit) {
                                            client.end();
                                        }
                                    });
                                });
                            }
                        } else if (msg.cmd === "reply") {
                            if (msg.replyId) {
                                const callback = this.hostCallbacks.get(msg.replyId);

                                if (callback) {
                                    callback(msg);
                                    this.hostCallbacks.delete(msg.replyId);
                                }
                            }
                        } else {
                            client.destroy(new Error("Invalid message"));
                        }
                    } catch {
                        client.destroy(new Error("Invalid message"));
                    }
                });
            });

            const tryConnect = (resolve: () => void) => {
                const client = net.createConnection(sockFile, () => {
                    client.write(JSON.stringify({ cmd: "connect", app: this.serverName }));
                    resolve();
                });
                client.on("data", (buf) => {
                    try {
                        const msg = JSON.parse(buf.toString()) as { cmd: string; replyId: string; };

                        if (msg.cmd === "reload") {
                            this.reload().then(() => {
                                let result: string;

                                if (this.serverName) {
                                    result = `gRPC app [${this.serverName}] reloaded`;
                                } else {
                                    result = `gRPC clients reloaded`;
                                }

                                client.write(JSON.stringify({
                                    cmd: "reply",
                                    replyId: msg.replyId,
                                    result
                                }));
                                this._onReload?.();
                            }).catch(console.error);
                        } else if (msg.cmd === "stop") {
                            this.stop().then(() => {
                                let result: string;

                                if (this.serverName) {
                                    result = `gRPC app [${this.serverName}] stopped`;
                                } else {
                                    result = "gRPC clients stopped";
                                }

                                client.write(JSON.stringify({
                                    cmd: "reply",
                                    replyId: msg.replyId,
                                    result
                                }));

                                if (this.host) {
                                    this.hostClients.forEach((_client) => {
                                        _client.end();
                                    });
                                    this.host.close(() => {
                                        this._onStop?.();
                                    });
                                } else {
                                    this._onStop?.();
                                }
                            }).catch(console.error);
                        }
                    } catch {
                        // ...
                    }
                }).on("end", () => {
                    if (!this.host) {
                        this.tryHost().catch(console.error);
                    }
                }).on("error", err => {
                    if (isSocketResetError(err) && !this.host) {
                        this.tryHost().catch(console.error);
                    } else {
                        reject(err);
                    }
                });
            };

            host.on("error", () => tryConnect(resolve));

            host.listen(sockFile, () => {
                this.host = host;
                tryConnect(resolve);
            });
        });
    }

    protected static async ensureDir(dirname: string) {
        try {
            await fs.promises.mkdir(dirname, { recursive: true });
        } catch (err) {
            if (err["code"] !== "EEXIST")
                throw err;
        }
    }

    protected static async unlinkIfExists(filename: string) {
        try {
            await fs.promises.unlink(filename);
        } catch (err) {
            if (err["code"] !== "ENOENT")
                throw err;
        }
    }

    protected static absPath(filename: string, withPipe = false): string {
        if (!/^\/|^[a-zA-Z]:[\\\/]/.test(filename) && typeof process === "object") {
            filename = path.resolve(process.cwd(), filename);
        }

        if (path?.sep) {
            filename = filename.replace(/\\|\//g, path.sep);
        }

        if (withPipe &&
            typeof process === "object" && process.platform === "win32" &&
            !/\\\\[.?]\\pipe\\/.test(filename)
        ) {
            filename = "\\\\?\\pipe\\" + filename;
        }

        return filename;
    }

    static async issue(cmd: "reload" | "stop", app = "", config = "") {
        const conf = this.loadConfig(config);
        const sockFile = BootApp.absPath(conf.sockFile || "boot.sock", true);

        await new Promise<void>((resolve, reject) => {
            const client = net.createConnection(sockFile, () => {
                client.write(JSON.stringify({ cmd, app }));
            }).on("data", (buf) => {
                try {
                    const reply = JSON.parse(buf.toString()) as {
                        result?: string;
                        error?: string;
                    };

                    if (reply.error) {
                        console.error(reply.error);
                    } else {
                        console.info(reply.result ?? null);
                    }
                } catch (err) {
                    console.error(err);
                }
            }).on("end", () => {
                resolve();
            }).once("error", reject);
        });
    }
}
