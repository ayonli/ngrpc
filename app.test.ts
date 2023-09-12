/// <reference path="./services/ExampleService.ts" />
import jsext from "@ayonli/jsext";
import { sleep } from "@ayonli/jsext/promise";
import { test } from "mocha";
import * as assert from "assert";
import * as fs from "fs/promises";
import * as path from "path";
import ngrpc from "./app";
import { spawnSync } from "child_process";

test("ngrpc.loadConfig", async () => {
    const config = await ngrpc.loadConfig();
    assert.ok(config.apps.length > 0);
});

test("ngrpc.localConfig with local config file", async () => {
    await fs.copyFile("ngrpc.json", "ngrpc.local.json");
    const [err, config] = await jsext.try(ngrpc.loadConfig());
    await fs.unlink("ngrpc.local.json");

    assert.ok(!err);
    assert.ok(config.apps.length > 0);
});

test("ngrpc.loadConfig with failure", async () => {
    await fs.rename("ngrpc.json", "ngrpc.jsonc");
    const [err, config] = await jsext.try(ngrpc.loadConfig());
    await fs.rename("ngrpc.jsonc", "ngrpc.json");

    const filename = path.join(process.cwd(), "ngrpc.json");
    assert.strictEqual(err.message, `unable to load config file: ${filename}`);
    assert.ok(!config);
});

test("ngrpc.loadConfigForPM2", async () => {
    const cfg = await ngrpc.loadConfig();
    const pm2Cfg = ngrpc.loadConfigForPM2();

    assert.ok(pm2Cfg.apps.length > 0);

    for (const pm2App of pm2Cfg.apps) {
        if (!pm2App.script) {
            throw new Error("the app's script cannot be empty");
        }

        const ext = path.extname(pm2App.script);
        const app = cfg.apps.find(item => item.name === pm2App.name);

        if (!app) {
            throw new Error(`app [${pm2App.name}] is not found`);
        }

        if (app.name.includes(" ")) {
            assert.strictEqual(pm2App.args, `"${app.name}"`);
        } else {
            assert.strictEqual(pm2App.args, app.name);
        }

        assert.deepStrictEqual(pm2App.env ?? {}, app.env ?? {});

        if (app.stdout && app.stderr) {
            assert.strictEqual(pm2App.out_file, app.stdout);
            assert.strictEqual(pm2App.err_file, app.stderr);
        } else if (app.stdout && !app.stderr) {
            assert.strictEqual(pm2App.log_file, app.stdout);
        }

        if (ext === ".js") {
            assert.strictEqual(pm2App.interpreter_args, "-r source-map-support/register");
        } else if (ext === ".ts") {
            assert.strictEqual(pm2App.interpreter_args, "-r ts-node/register");
        } else if (ext === ".go") {
            assert.strictEqual(pm2App.interpreter, "go");
            assert.strictEqual(pm2App.interpreter_args, "run");
        } else if (ext === ".exe" || !ext) {
            assert.strictEqual(pm2App.interpreter, "none");
        } else {
            throw new Error(`entry file '${app.entry}' of app [${app.name}] is recognized`);
        }
    }
});

test("ngrpc.start", jsext.func(async (defer) => {
    const app = await ngrpc.start("example-server");
    defer(() => app.stop());

    assert.strictEqual(app.name, "example-server");

    const reply = await services.ExampleService.sayHello({ name: "World" });
    assert.strictEqual(reply.message, "Hello, World");
}));

test("ngrpc.start without app name", jsext.func(async function (defer) {
    this.timeout(5_000);

    spawnSync("ngrpc", ["start", "example-server"]);
    defer(async () => {
        spawnSync("ngrpc", ["stop"]);
        await sleep(10);
    });

    const app = await ngrpc.start();
    defer(() => app.stop());

    const reply = await services.ExampleService.sayHello({ name: "World" });
    assert.strictEqual(reply.message, "Hello, World");
}));


test("ngrpc.startWithConfig", jsext.func(async (defer) => {
    const cfg = await ngrpc.loadConfig();
    const app = await ngrpc.startWithConfig("example-server", cfg);
    defer(() => app.stop());

    assert.strictEqual(app.name, "example-server");

    const reply = await services.ExampleService.sayHello({ name: "World" });
    assert.strictEqual(reply.message, "Hello, World");
}));

test("ngrpc.startWithConfig with xds protocol", async () => {
    const cfg = await ngrpc.loadConfig();
    const cfgApp = cfg.apps.find(item => item.name === "example-server");

    if (!cfgApp) {
        throw new Error("app [example-server] not found");
    }

    cfgApp.entry = "entry/main.ts";
    cfgApp.url = "xds://localhost:4000";

    const [err, app] = await jsext.try(ngrpc.startWithConfig("example-server", cfg));

    assert.ok(!app);
    assert.strictEqual(err.message,
        `app [example-server] cannot be served since it uses 'xds:' protocol`);
});

test("ngrpc.start invalid app", async () => {
    const [err, app] = await jsext.try(ngrpc.start("test-server"));

    assert.ok(!app);
    assert.strictEqual(err.message, "app [test-server] is not configured");
});

test("ngrpc.startWithConfig with invalid URl", async () => {
    const cfg = await ngrpc.loadConfig();
    const cfgApp = cfg.apps.find(item => item.name === "example-server");

    if (!cfgApp) {
        throw new Error("app [example-server] not found");
    }

    cfgApp.entry = "entry/main.ts";
    cfgApp.url = "grpc://localhost:abc";

    const [err, app] = await jsext.try(ngrpc.startWithConfig("example-server", cfg));

    assert.ok(!app);
    assert.strictEqual(err.message, `Invalid URL`);
});

test("ngrpc.start duplicated call", jsext.func(async (defer) => {
    const app1 = await ngrpc.start("example-server");
    const [err, app2] = await jsext.try(ngrpc.start("post-server"));
    defer(() => app1.stop());

    assert.ok(!app2);
    assert.strictEqual(err.message, "an app is already running");
}));

test("ngrpc.getServiceClient", jsext.func(async (defer) => {
    const app = await ngrpc.start("post-server");
    defer(() => app.stop());

    const ins1 = ngrpc.getServiceClient("services.PostService");
    const ins2 = ngrpc.getServiceClient("services.PostService", "post-server");
    const ins3 = ngrpc.getServiceClient("services.PostService", "grpcs://localhost:4002");

    assert.ok(!!ins1);
    assert.ok(!!ins2);
    assert.ok(!!ins3);
}));

test("ngrpc.runSnippet", jsext.func(async function (defer) {
    this.timeout(5_000);

    spawnSync("ngrpc", ["start", "example-server"]);
    defer(async () => {
        spawnSync("ngrpc", ["stop"]);
        await sleep(10);
    });

    let message: string | undefined;
    await ngrpc.runSnippet(async () => {
        const reply = await services.ExampleService.sayHello({ name: "World" });
        message = reply.message;
    });

    assert.strictEqual(message, "Hello, World");
}));

test("app.stop and app.onStop", async () => {
    const app = await ngrpc.start("example-server");
    let stopped = false;

    app.onStop(() => {
        stopped = true;
    });

    await app.stop();
    assert.ok(stopped);
});

test("app.reload and app.onReload", jsext.func(async (defer) => {
    const app = await ngrpc.start("example-server");
    defer(() => app.stop());

    let reloaded = false;

    app.onReload(() => {
        reloaded = true;
    });

    await app.reload();
    assert.ok(reloaded);
}));
