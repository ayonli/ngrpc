/// <reference path="./services/ExampleService.ts" />
/// <reference path="./services/UserService.d.ts" />
/// <reference path="./services/PostService.ts" />

import { deepStrictEqual, ok } from "assert";
import { it } from "mocha";
import App from ".";
import { spawn, execSync, ChildProcess, SpawnOptions } from "child_process";
import { unlinkSync } from "fs";
import * as fs from "fs/promises";
import * as path from "path";

const isTsNode = !!process[Symbol.for("ts-node.register.instance")];

async function runCliCommand(cmd: string, args: string[], options: SpawnOptions = {}) {
    let child: ChildProcess;

    // Use spawn instead of spawnSync to prevent blocking the thread.
    if (isTsNode) {
        child = spawn("npx", ["ts-node", "cli.ts", cmd, ...args], options);
    } else {
        child = spawn("node", ["cli.js", cmd, ...args], options);
    }

    await new Promise<void>((resolve, reject) => {
        child.once("exit", () => resolve())
            .once("err", reject);
    });
}

let goProcess: ChildProcess;

before(function (done) {
    // Must build the go program before running it, otherwise the
    // goProcess.kill() won"t be able to release the port, since
    // the server process isn't the real process the start the gRPC server
    // and when the go process is killed, the real process the holds the port
    // still hangs and hangs the Node.js process as well, reason is unknown.
    this.timeout(120_000); // this could take a while for go installing dependencies
    execSync("go build -o user-server main.go", { cwd: __dirname });
    goProcess = spawn(__dirname + "/user-server");
    goProcess.on("spawn", () => {
        done();
    }).on("error", err => {
        done(err);
    });
});

after(function () {
    goProcess.kill();

    setTimeout(() => {
        unlinkSync(__dirname + "/user-server");
        process.exit();
    });
});

describe("App.boot", () => {
    it("App.boot(app)", async () => {
        let app: App;

        try {
            app = await App.boot("example-server");
            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });
            await app.stop();
        } catch (err) {
            await app?.stop();
            throw err;
        }
    });

    it("App.boot(app, config)", async () => {
        let app: App;

        try {
            app = await App.boot("test-server", "test.config.json");
            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });
            await app.stop();
        } catch (err) {
            await app?.stop();
            throw err;
        }
    });

    it("App.boot()", async function () {
        this.timeout(20_000);

        await runCliCommand("start", ["-d"], { stdio: "inherit" });

        try {
            await App.boot();
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

            await App.sendCommand("stop");
        } catch (err) {
            await App.sendCommand("stop");
            throw err;
        }
    });

    it("App.boot(null, config)", async function () {
        this.timeout(20_000);

        await runCliCommand("start", ["-d", "-c", "test.config.json"], { stdio: "inherit" });

        try {
            await App.boot(null, "test.config.json");
            const reply = await services.ExampleService.sayHello({ name: "World" });

            deepStrictEqual(reply, { message: "Hello, World" });

            await App.sendCommand("stop");
        } catch (err) {
            await App.sendCommand("stop");
            throw err;
        }
    });
});

describe("app.[method]", () => {
    it("app.stop()", async function () {
        this.timeout(20_000); // prolong test for gRPC connection timeout
        let app: App;
        let reply: any;
        let reply2: any;
        let err: Error;

        try {
            app = await App.boot("example-server");
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
        deepStrictEqual(String(err), "Error: Failed to connect before the deadline");
    });

    it("app.reload()", async function () {
        this.timeout(20_000);

        let app: App;
        let reply: any;
        let reply2: any;

        const filename = path.join(__dirname, "services", "ExampleService" + (isTsNode ? ".ts" : ".js"));
        let contents = await fs.readFile(filename, "utf8");

        try {
            app = await App.boot("example-server");
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
        let app: App;
        let log: string;

        try {
            app = await App.boot("example-server");
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
        let app: App;
        let log: string;

        try {
            app = await App.boot("example-server");
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

describe("App.loadConfig*", () => {
    it("App.loadConfig()", async () => {
        const conf = App.loadConfig();
        const conf1 = require("./boot.config.json");

        conf1.protoOptions.longs = String;

        deepStrictEqual(conf, conf1);
    });

    it("App.loadConfig(config)", () => {
        const conf = App.loadConfig("./test.config.json");
        const conf1 = require("./test.config.json");

        conf1.protoOptions.longs = String;

        deepStrictEqual(conf, conf1);
    });

    it("App.loadConfigForPM2()", async () => {
        const conf = App.loadConfigForPM2();

        deepStrictEqual(conf, {
            apps: [
                {
                    name: "post-server",
                    script: "./main",
                    log_file: "./out.log",
                    env: void 0,
                }
            ]
        });
    });
});

describe("App.runSnippet", () => {
    it("App.runSnippet(fn)", async function () {
        this.timeout(20_000);

        await runCliCommand("start", ["-d"], { stdio: "inherit" });

        let reply: any;

        await App.runSnippet(async () => {
            reply = await services.ExampleService.sayHello({ name: "World" });
        });
        await App.sendCommand("stop");

        deepStrictEqual(reply, { message: "Hello, World" });
    });

    it("App.runSnippet(fn, config)", async function () {
        this.timeout(20_000);

        await runCliCommand("start", ["-d", "-c", "test.config.json"]);

        let reply: any;

        await App.runSnippet(async () => {
            reply = await services.ExampleService.sayHello({ name: "World" });
        }, "test.config.json");
        await App.sendCommand("stop");

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
        ok((await fs.stat(path.join(testDir, "boot.config.json"))).isFile());
        ok((await fs.stat(path.join(testDir, "services"))).isDirectory());
        ok((await fs.stat(path.join(testDir, "services", "ExampleService.proto"))).isFile());
        ok((await fs.stat(path.join(testDir, "services", "ExampleService.ts"))).isFile());

        await fs.rm(testDir, { recursive: true });
    });

    it("init <package>", async () => {
        await fs.mkdir(testDir);

        await runInitCommandInTestDir(["grpc"]);

        const confFile = path.join(testDir, "boot.config.json");
        const protoFile = path.join(testDir, "grpc", "ExampleService.proto");
        const tsFile = path.join(testDir, "grpc", "ExampleService.ts");

        ok((await fs.stat(testDir)).isDirectory());
        ok((await fs.stat(path.join(testDir, "tsconfig.json"))).isFile());
        ok((await fs.stat(confFile)).isFile());
        ok((await fs.stat(path.join(testDir, "grpc"))).isDirectory());
        ok((await fs.stat(protoFile)).isFile());
        ok((await fs.stat(tsFile)).isFile());

        const conf = JSON.parse(await fs.readFile(confFile, "utf8"));
        deepStrictEqual(conf.package, "grpc");
        deepStrictEqual(conf.apps, [
            {
                "name": "example-server",
                "uri": "grpc://localhost:4000",
                "serve": true,
                "services": [
                    `grpc.ExampleService`
                ]
            }
        ]);

        const proto = await fs.readFile(protoFile, "utf8");
        ok(proto.includes("package grpc;"));

        const ts = await fs.readFile(tsFile, "utf8");
        ok(ts.includes("namespace grpc {"));

        await fs.rm(testDir, { recursive: true });
    });

    it("init --config <filename>", async () => {
        await fs.mkdir(testDir);

        await runInitCommandInTestDir(["-c", "test.config.json"]);

        ok((await fs.stat(path.join(testDir, "test.config.json"))).isFile());

        await fs.rm(testDir, { recursive: true });
    });

    it("init --config <filename> <package>", async () => {
        await fs.mkdir(testDir);

        await runInitCommandInTestDir(["-c", "test.config.json", "grpc"]);

        const confFile = path.join(testDir, "test.config.json");
        ok((await fs.stat(confFile)).isFile());

        const conf = JSON.parse(await fs.readFile(confFile, "utf8"));
        deepStrictEqual(conf.package, "grpc");
        deepStrictEqual(conf.apps, [
            {
                "name": "example-server",
                "uri": "grpc://localhost:4000",
                "serve": true,
                "services": [
                    `grpc.ExampleService`
                ]
            }
        ]);

        await fs.rm(testDir, { recursive: true });
    });
});

describe("CLI:start", () => {
    it("start <app>", async function () {
        this.timeout(20_000);

        let child: ChildProcess;
        let app: App;

        try {
            if (isTsNode) {
                child = spawn("npx",
                    ["ts-node", "cli.ts", "start", "example-server"],
                    { stdio: "inherit" });
            } else {
                child = spawn("node", ["cli.js", "start", "example-server"], { stdio: "inherit" });
            }

            await new Promise<void>((resolve, reject) => {
                child.once("spawn", () => resolve())
                    .once("error", reject);
            });
            await new Promise((resolve) => setTimeout(resolve, 2_000)); // wait a while

            app = await App.boot();
            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });

            await app.stop();
            child.kill();
        } catch (err) {
            await app?.stop();
            child?.kill();
            throw err;
        }
    });

    it("start --detach <app>", async function () {
        this.timeout(20_000);

        try {
            await runCliCommand("start", ["-d", "example-server"], { stdio: "inherit" });
            await App.boot();

            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });

            await App.sendCommand("stop", "example-server");
        } catch (err) {
            await App.sendCommand("stop");
            throw err;
        }
    });

    it("start --detach", async function () {
        this.timeout(20_000);

        try {
            await runCliCommand("start", ["-d"]);
            await App.boot();

            const reply = await services.ExampleService.sayHello({ name: "World" });
            deepStrictEqual(reply, { message: "Hello, World" });

            await App.sendCommand("stop");
        } catch (err) {
            await App.sendCommand("stop");
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
            await runCliCommand("start", ["-d"], { stdio: "inherit" });
            await App.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCliCommand("restart", [], { stdio: "inherit" });

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await App.sendCommand("stop");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await App.sendCommand("stop");
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
            await runCliCommand("start", ["-d", "example-server"], { stdio: "inherit" });
            const app = await App.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCliCommand("restart", ["example-server"], { stdio: "inherit" });

            // gRPC's reconnect algorithm takes too long, we just manually reload our clients.
            await app.reload();

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await App.sendCommand("stop", "example-server");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await App.sendCommand("stop");
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
            await runCliCommand("start", ["-d"], { stdio: "inherit" });
            await App.boot();
            await App.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCliCommand("reload", [], { stdio: "inherit" });

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await App.sendCommand("stop");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await App.sendCommand("stop");
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
            await runCliCommand("start", ["-d", "example-server"], { stdio: "inherit" });
            await App.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            const newContents = contents.replace("Hello, ", "Hi, ");
            await fs.writeFile(filename, newContents, "utf8"); // update the file

            await runCliCommand("reload", ["example-server"], { stdio: "inherit" });

            reply2 = await services.ExampleService.sayHello({ name: "World" });

            await fs.writeFile(filename, contents, "utf8"); // recover the file

            deepStrictEqual(reply, { message: "Hello, World" });
            deepStrictEqual(reply2, { message: "Hi, World" });

            await App.sendCommand("stop", "example-server");
        } catch (err) {
            await fs.writeFile(filename, contents, "utf8"); // recover the file
            await App.sendCommand("stop");
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
            await runCliCommand("start", ["-d"], { stdio: "inherit" });
            await App.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            await runCliCommand("stop", [], { stdio: "inherit" });

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
            await runCliCommand("start", ["-d", "example-server"], { stdio: "inherit" });
            await App.boot();

            reply = await services.ExampleService.sayHello({ name: "World" });

            await runCliCommand("stop", ["example-server"], { stdio: "inherit" });

            await services.ExampleService.sayHello({ name: "World" });
        } catch (_err) {
            err = _err;
        }

        deepStrictEqual(reply, { message: "Hello, World" });
        deepStrictEqual(String(err), "Error: Failed to connect before the deadline");
    });
});
