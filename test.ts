/// <reference path="./services/ExampleService.ts" />
/// <reference path="./services/UserService.ts" />
/// <reference path="./services/PostService.ts" />

import { deepStrictEqual, ok } from "assert";
import { it } from "mocha";
import ngrpc, { RpcApp } from ".";
import { isTsNode } from "./util";
import { spawn, execSync, ChildProcess, SpawnOptions } from "child_process";
import { unlinkSync } from "fs";
import * as fs from "fs/promises";
import * as path from "path";
import cloneDeep = require("lodash/cloneDeep");

async function runCommand(cmd: string, args: string[] = [], options: SpawnOptions = {}) {
    let child: ChildProcess;

    // Use spawn instead of spawnSync to prevent blocking the thread.
    if (isTsNode) {
        child = spawn("npx", ["ts-node", "cli.ts", cmd, ...args], { stdio: "inherit", ...options });
    } else {
        child = spawn("node", ["cli.js", cmd, ...args], { stdio: "inherit", ...options });
    }

    await new Promise<void>((resolve, reject) => {
        child.once("exit", () => resolve())
            .once("err", reject);
    });
}

let goProcess: ChildProcess;

before(async function () {
    // Must build the go program before running it, otherwise the
    // goProcess.kill() won"t be able to release the port, since
    // the server process isn't the real process the start the gRPC server
    // and when the go process is killed, the real process the holds the port
    // still hangs and hangs the Node.js process as well, reason is unknown.
    this.timeout(120_000); // this could take a while for go installing dependencies
    execSync("go build -o user-server main.go", { cwd: __dirname });
    goProcess = spawn(__dirname + "/user-server");

    await new Promise<void>((resolve, reject) => {
        goProcess.on("spawn", () => {
            resolve();
        }).on("error", err => {
            reject(err);
        });
    });
});

after(async function () {
    goProcess.kill("SIGINT");

    setTimeout(() => {
        unlinkSync(__dirname + "/user-server");
        process.exit();
    });
});

describe("ngrpc.boot", () => {
    it("ngrpc.boot(app)", async () => {
        let app: RpcApp | undefined;

        try {
            app = await ngrpc.boot("example-server");
            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });
            await app.stop();
        } catch (err) {
            await app?.stop();
            throw err;
        }
    });

    it("ngrpc.boot()", async function () {
        this.timeout(20_000);

        await runCommand("start");

        try {
            await ngrpc.boot();
            const post = await services.PostService.getPost({ id: 1 });

            deepStrictEqual(post, {
                id: 1,
                title: "My first article",
                description: "This is my first article",
                content: "The article contents ...",
                author: {
                    id: "ayon.li",
                    name: "A-yon Lee",
                    gender: 1,
                    age: 28,
                    email: "the@ayon.li",
                },
                "_author": "author",
                "_description": "description",
            });

            const result = await services.UserService.getMyPosts({ id: "ayon.li" });

            deepStrictEqual(result, {
                posts: [
                    {
                        id: 1,
                        title: "My first article",
                        description: "This is my first article",
                        content: "The article contents ...",
                        author: {
                            id: "ayon.li",
                            name: "A-yon Lee",
                            gender: 1,
                            age: 28,
                            email: "the@ayon.li",
                        },
                        "_author": "author",
                        "_description": "description",
                    },
                    {
                        id: 2,
                        title: "My second article",
                        description: "This is my second article",
                        content: "The article contents ...",
                        author: {
                            id: "ayon.li",
                            name: "A-yon Lee",
                            gender: 1,
                            age: 28,
                            email: "the@ayon.li",
                        },
                        "_author": "author",
                        "_description": "description",
                    }
                ]
            })

            await ngrpc.sendCommand("stop");
        } catch (err) {
            await ngrpc.sendCommand("stop");
            throw err;
        }
    });
});

describe("app.[method]", () => {
    it("app.stop()", async function () {
        this.timeout(20_000); // prolong test for gRPC connection timeout
        let app: RpcApp | undefined;
        let reply: any | undefined;
        let reply2: any | undefined;
        let err: Error | undefined;

        try {
            app = await ngrpc.boot("example-server");
            reply = await services.ExampleService.sayHello({ name: "World" });
            await app.stop();

            // This call will fail since the app is stopped.
            reply2 = await services.ExampleService.sayHello({ name: "World" });
        } catch (_err) {
            err = _err;
            app?.stop();
        }

        deepStrictEqual(reply, { message: "Hello, World" });
        deepStrictEqual(reply2, void 0);
        deepStrictEqual(String(err), "Error: service services.ExampleService is not available");
    });

    it("app.reload()", async function () {
        this.timeout(20_000);

        let app: RpcApp | undefined;
        let reply: any | undefined;
        let reply2: any | undefined;

        const filename = path.join(__dirname, "services", "ExampleService" + (isTsNode ? ".ts" : ".js"));
        let contents = await fs.readFile(filename, "utf8");

        try {
            app = await ngrpc.boot("example-server");
            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await app.reload();

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await app.stop();
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            app?.stop();
            throw err;
        }

        deepStrictEqual(reply, { message: "Hello, World" });
        deepStrictEqual(reply2, { message: "Hi, World" });
    });

    it("app.onReload(callback)", async () => {
        let app: RpcApp | undefined;
        let log: string | undefined;

        try {
            app = await ngrpc.boot("example-server");
            app.onReload(() => {
                log = "example-server has been reloaded";
            });

            await app.reload();
            await app.stop();
        } catch (err) {
            app?.stop();
            throw err;
        }

        deepStrictEqual(log, "example-server has been reloaded");
    });

    it("app.onStop(callback)", async () => {
        let app: RpcApp | undefined;
        let log: string | undefined;

        try {
            app = await ngrpc.boot("example-server");
            app.onStop(() => {
                log = "example-server has been stopped";
            });

            await app.stop();
        } catch (err) {
            app?.stop();
            throw err;
        }

        deepStrictEqual(log, "example-server has been stopped");
    });
});

describe("ngrpc.loadConfig*", () => {
    it("ngrpc.loadConfig()", async () => {
        const conf = await ngrpc.loadConfig();
        const conf1 = require("./ngrpc.json");

        conf1.protoOptions.longs = String;

        deepStrictEqual(conf, conf1);
    });

    it("ngrpc.loadConfigForPM2()", async () => {
        const conf = await ngrpc.loadConfigForPM2();

        deepStrictEqual(conf, {
            apps: [
                {
                    name: "example-server",
                    script: path.join(__dirname, "cli.js"),
                    args: `example-server`,
                    env: {},
                    log_file: "./out.log",
                },
                {
                    name: "post-server",
                    script: path.join(__dirname, "cli.js"),
                    args: `post-server`,
                    env: {},
                    log_file: "./out.log",
                }
            ]
        });
    });
});

describe("ngrpc.runSnippet", () => {
    it("ngrpc.runSnippet(fn)", async function () {
        this.timeout(20_000);

        await runCommand("start");

        let reply: any;

        await ngrpc.runSnippet(async () => {
            reply = await services.ExampleService.sayHello({ name: "World" });
        });
        await ngrpc.sendCommand("stop");

        deepStrictEqual(reply, { message: "Hello, World" });
    });
});

describe("CLI:init", () => {
    // Use child process for CLI tests
    const testDir = path.join(process.cwd(), "test");
    const spawnOptions: SpawnOptions = { cwd: testDir, stdio: "inherit" };

    async function runInitCommandInTestDir(args: string[]) {
        let child: ChildProcess;

        // Use spawn instead of spawnSync to prevent blocking the thread.
        if (isTsNode) {
            child = spawn("npx", ["ts-node", "../cli.ts", "init", ...args], spawnOptions);
        } else {
            child = spawn("node", ["../cli.js", "init", ...args], spawnOptions);
        }

        await new Promise<void>((resolve, reject) => {
            child.once("exit", () => resolve())
                .once("err", reject);
        });
    }

    it("init", async () => {
        await fs.mkdir(testDir);

        await runInitCommandInTestDir([]);

        ok((await fs.stat(testDir)).isDirectory());
        ok((await fs.stat(path.join(testDir, "tsconfig.json"))).isFile());
        ok((await fs.stat(path.join(testDir, "ngrpc.json"))).isFile());
        ok((await fs.stat(path.join(testDir, "proto"))).isDirectory());
        ok((await fs.stat(path.join(testDir, "services"))).isDirectory());
        ok((await fs.stat(path.join(testDir, "proto", "ExampleService.proto"))).isFile());
        ok((await fs.stat(path.join(testDir, "services", "ExampleService.ts"))).isFile());

        await fs.rm(testDir, { recursive: true });
    });
});

describe("CLI:start", () => {
    it("start <app>", async function () {
        this.timeout(20_000);

        try {
            await runCommand("start", ["example-server"]);
            await ngrpc.boot();

            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });

            await ngrpc.sendCommand("stop", "example-server");
        } catch (err) {
            await ngrpc.sendCommand("stop");
            throw err;
        }
    });

    it("start", async function () {
        this.timeout(20_000);

        try {
            await runCommand("start");
            await ngrpc.boot();

            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });

            await ngrpc.sendCommand("stop");
        } catch (err) {
            await ngrpc.sendCommand("stop");
            throw err;
        }
    });
});

describe("CLI:restart", () => {
    it("restart", async function () {
        this.timeout(20_000);

        let reply: any;
        let reply2: any;

        const filename = path.join(__dirname, "services", "ExampleService" + (isTsNode ? ".ts" : ".js"));
        let contents = await fs.readFile(filename, "utf8");

        try {
            await runCommand("start");
            const app = await ngrpc.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCommand("restart");

            await app.reload();
            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await ngrpc.sendCommand("stop");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await ngrpc.sendCommand("stop");
            throw err;
        }
    });

    it("restart <app>", async function () {
        this.timeout(20_000); // set long enough for gRPC to reconnect :(

        let reply: any;
        let reply2: any;

        const filename = path.join(__dirname, "services", "ExampleService" + (isTsNode ? ".ts" : ".js"));
        let contents = await fs.readFile(filename, "utf8");

        try {
            await runCommand("start", ["example-server"]);
            const app = await ngrpc.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCommand("restart", ["example-server"]);

            // gRPC's reconnect algorithm takes too long, we just manually reload our clients.
            await app.reload();

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await ngrpc.sendCommand("stop", "example-server");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await ngrpc.sendCommand("stop");
            throw err;
        }
    });
});

describe("CLI:reload", () => {
    it("reload", async function () {
        this.timeout(20_000);

        let reply: any;
        let reply2: any;

        const filename = path.join(__dirname, "services", "ExampleService" + (isTsNode ? ".ts" : ".js"));
        let contents = await fs.readFile(filename, "utf8");

        try {
            await runCommand("start");
            await ngrpc.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCommand("reload");

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await ngrpc.sendCommand("stop");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await ngrpc.sendCommand("stop");
            throw err;
        }
    });

    it("reload <app>", async function () {
        this.timeout(20_000);

        let reply: any;
        let reply2: any;

        const filename = path.join(__dirname, "services", "ExampleService" + (isTsNode ? ".ts" : ".js"));
        let contents = await fs.readFile(filename, "utf8");

        try {
            await runCommand("start", ["example-server"]);
            await ngrpc.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCommand("reload", ["example-server"]);

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await ngrpc.sendCommand("stop", "example-server");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await ngrpc.sendCommand("stop");
            throw err;
        }
    });
});


describe("CLI:stop", () => {
    it("stop", async function () {
        this.timeout(20_000);

        let reply: any;
        let err: any;

        try {
            await runCommand("start");
            const app = await ngrpc.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            await runCommand("stop");

            await app.reload();
            await services.ExampleService.sayHello({ name: "World" });
        } catch (_err) {
            err = _err;
        }

        deepStrictEqual(reply, { message: "Hello, World" });
        deepStrictEqual(String(err), "Error: Failed to connect before the deadline");
    });

    it("stop <app>", async function () {
        this.timeout(20_000);

        let reply: any;
        let err: any;

        try {
            await runCommand("start", ["example-server"]);
            const app = await ngrpc.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            await runCommand("stop", ["example-server"]);

            await app.reload();
            await services.ExampleService.sayHello({ name: "World" });
        } catch (_err) {
            err = _err;
        }

        deepStrictEqual(reply, { message: "Hello, World" });
        deepStrictEqual(String(err), "Error: Failed to connect before the deadline");
    });
});

describe("config options", () => {
    const testDir = path.join(process.cwd(), "test");

    async function runCommandInTestDir(cmd: string, args: string[] = []) {
        let child: ChildProcess;

        // Use spawn instead of spawnSync to prevent blocking the thread.
        if (isTsNode) {
            child = spawn("npx", ["ts-node", "../cli.ts", cmd, ...args], {
                cwd: path.join(process.cwd(), "test"),
                stdio: "inherit",
            });
        } else {
            child = spawn("node", ["../cli.js", cmd, ...args], {
                cwd: path.join(process.cwd(), "test"),
                stdio: "inherit",
            });
        }

        await new Promise<void>((resolve, reject) => {
            child.once("exit", () => resolve())
                .once("err", reject);
        });
    }

    before(async function () {
        this.timeout(20_000);
        await fs.mkdir(testDir);

        await runCommandInTestDir("init");

        const mainTs = (await fs.readFile("main.ts", "utf8"))
            .replace(`"."`, `"@ayonli/ngrpc"`);
        await fs.writeFile("test/main.ts", mainTs, "utf8");

        const conf = JSON.parse(await fs.readFile("test/ngrpc.json", "utf8"));
        conf.entry = "dist/main.js";
        conf.importRoot = "dist";
        await fs.writeFile("test/ngrpc.json", JSON.stringify(conf, null, "    "), "utf8");

        const tsConf = JSON.parse(await fs.readFile("test/tsconfig.json", "utf8"));
        tsConf.compilerOptions.outDir = "dist";
        await fs.writeFile("test/tsconfig.json", JSON.stringify(tsConf, null, "    "), "utf8");

        if (isTsNode) {
            await (async function compileMainProject() {
                const mainTsConfContents = await fs.readFile("tsconfig.json", "utf8");
                const mainTsConf = JSON.parse(mainTsConfContents);
                mainTsConf.compilerOptions.outDir = "dist";
                await fs.writeFile("tsconfig.json", JSON.stringify(mainTsConf, null, "    "), "utf8");

                const child = spawn("npx", ["tsc"], { stdio: "inherit" });
                await new Promise<void>((resolve, reject) => {
                    child.once("exit", () => resolve())
                        .once("err", reject);
                });

                await fs.writeFile("tsconfig.json", mainTsConfContents, "utf8");
            })();
        }

        await (async function compileTest() {
            try {
                const child = spawn("npx", ["tsc"], {
                    stdio: "ignore",
                    cwd: path.join(process.cwd(), "test"),
                });
                await new Promise<void>((resolve, reject) => {
                    child.once("exit", () => resolve())
                        .once("err", reject);
                });
            } catch (err) {
                // ignore
            }

            { // rewrite entry file
                let mainJs = await fs.readFile("test/dist/main.js", "utf8");

                if (isTsNode) {
                    mainJs = mainJs.replace(`"@ayonli/ngrpc"`, `"../../dist"`);
                } else {
                    mainJs = mainJs.replace(`"@ayonli/ngrpc"`, `"../.."`);
                }

                await fs.writeFile("test/dist/main.js", mainJs, "utf8");
            }

            { // rewrite ExampleService
                let exampleService = await fs.readFile("test/dist/services/ExampleService.js", "utf8");

                if (isTsNode) {
                    exampleService = exampleService.replace(`"@ayonli/ngrpc"`, `"../../../dist/util"`);
                } else {
                    exampleService = exampleService.replace(`"@ayonli/ngrpc"`, `"../../../util"`);
                }

                await fs.writeFile("test/dist/services/ExampleService.js", exampleService, "utf8");
            }
        })();

        const conf2 = cloneDeep(conf);

        conf2.entry = "test/dist/main.js";
        conf2.importRoot = "test/dist";
        conf2.protoPaths = ["test/proto"];

        await runCommandInTestDir("start");
    });

    after(async function () {
        this.timeout(20_000);
        await runCommandInTestDir("stop");
        await fs.rm(testDir, { recursive: true });

        if (isTsNode) {
            await fs.rm("dist", { recursive: true });
        }
    });

    it("run the example", async function () {
        this.timeout(20_000);
        let app: RpcApp | undefined;

        try {
            app = await ngrpc.boot();

            // @ts-ignore
            const reply = await services.ExampleService.sayHello({ name: "World" });

            deepStrictEqual(reply, { message: "Hello, World" });
            await app.stop();
        } catch (err) {
            await app?.stop();
            throw err;
        }
    });
});
