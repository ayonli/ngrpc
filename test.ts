/// <reference path="./services/ExampleService.ts" />
/// <reference path="./services/UserService.d.ts" />
/// <reference path="./services/PostService.ts" />

import { deepStrictEqual } from "assert";
import { it } from "mocha";
import App from ".";
import { spawn, spawnSync, execSync, ChildProcess } from "child_process";
import { unlinkSync } from "fs";
import * as fs from "fs/promises";
import * as path from "path";

const isTsNode = !!process[Symbol.for("ts-node.register.instance")];

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

        // Use a child process to start the app so that the clients won't be connected automatically.
        if (isTsNode) {
            spawnSync("npx", ["ts-node", "cli.ts", "start", "-d"]);
        } else {
            spawnSync("node", ["cli.js", "start", "-d"]);
        }

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

        // Use a child process to start the app so that the clients won't be connected automatically.
        if (isTsNode) {
            spawnSync("npx", ["ts-node", "cli.ts", "start", "-d", "-c", "test.config.json"]);
        } else {
            spawnSync("node", ["cli.js", "start", "-d", "-c", "test.config.json"]);
        }

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

        // Use a child process to start the app so that the clients won't be connected automatically.
        if (isTsNode) {
            spawnSync("npx", ["ts-node", "cli.ts", "start", "-d"]);
        } else {
            spawnSync("node", ["cli.js", "start", "-d"]);
        }

        let reply: any;

        await App.runSnippet(async () => {
            reply = await services.ExampleService.sayHello({ name: "World" });
        });
        await App.sendCommand("stop");

        deepStrictEqual(reply, { message: "Hello, World" });
    });

    it("App.runSnippet(fn, config)", async function () {
        this.timeout(20_000);

        // Use a child process to start the app so that the clients won't be connected automatically.
        if (isTsNode) {
            spawnSync("npx", ["ts-node", "cli.ts", "start", "-d", "-c", "test.config.json"]);
        } else {
            spawnSync("node", ["cli.js", "start", "-d", "-c", "test.config.json"]);
        }

        let reply: any;

        await App.runSnippet(async () => {
            reply = await services.ExampleService.sayHello({ name: "World" });
        }, "test.config.json");
        await App.sendCommand("stop");

        deepStrictEqual(reply, { message: "Hello, World" });
    });
});
