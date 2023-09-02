/// <reference path="./services/ExampleService.ts" />
/// <reference path="./services/UserService.ts" />
/// <reference path="./services/PostService.ts" />

import { deepStrictEqual } from "assert";
import { it } from "mocha";
import ngrpc, { RpcApp } from ".";
import { isTsNode } from "./util";
import { spawn, execSync, ChildProcess, SpawnOptions } from "child_process";
import { unlinkSync } from "fs";
import * as fs from "fs/promises";
import * as path from "path";

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
            });

            await runCommand("stop");
        } catch (err) {
            await runCommand("stop");
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
        await runCommand("stop");

        deepStrictEqual(reply, { message: "Hello, World" });
    });
});
