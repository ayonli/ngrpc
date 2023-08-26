#!/usr/bin/env node
try {
    require("source-map-support/register");
} catch { }
import * as commander from "commander";
import * as fs from "fs/promises";
import * as path from "path";
import pkg = require("./package.json");
import ngrpc, { Config } from ".";
import { absPath, ensureDir, exists, spawnProcess } from "./util";

const program = new commander.Command("ngrpc");

program.description("start, reload or stop apps")
    .version(pkg.version);

program.command("init")
    .description("initiate a new gRPC project")
    .action(async () => {
        const tsConfig = absPath("tsconfig.json");
        const config = absPath("ngrpc.json");
        const servicesDir = absPath("services");
        const protoDir = absPath("proto");

        if (await exists(tsConfig)) {
            console.warn(`file '${path.basename(config)}' already exists`);
        } else {
            const tsConf = {
                "compilerOptions": {
                    "module": "commonjs",
                    "target": "es2018",
                    "newLine": "LF",
                    "importHelpers": true,
                    "noUnusedParameters": true,
                    "noUnusedLocals": true,
                    "noImplicitThis": true,
                    "sourceMap": true,
                    "declaration": true
                },
                "include": [
                    "*.ts",
                    "**/*.ts"
                ],
            };

            await fs.writeFile(tsConfig, JSON.stringify(tsConf, null, "    "), "utf8");
            console.info(`tsconfig file written to '${path.basename(tsConfig)}'`);
        }

        if (await exists(config)) {
            console.warn(`file '${path.basename(config)}' already exists`);
        } else {
            const conf: Config = {
                "$schema": "./node_modules/ngrpc/ngrpc.schema.json",
                "namespace": "services",
                "protoPaths": ["./proto"],
                "protoOptions": {
                    // @ts-ignore
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
                    }
                ]
            };
            await fs.writeFile(config, JSON.stringify(conf, null, "    "), "utf8");
            console.info(`config file written to '${path.basename(config)}'`);
        }

        if (await exists(servicesDir)) {
            console.warn(`path '${servicesDir}' already exists`);
        } else {
            await ensureDir(servicesDir);
            console.info(`path '${servicesDir}' created`);
        }

        if (await exists(protoDir)) {
            console.warn(`path '${protoDir}' already exists`);
        } else {
            await ensureDir(protoDir);
            console.info(`path '${protoDir}' created`);
        }

        const exampleProtoSrc = path.join(__dirname, "proto", "ExampleService.proto");
        const exampleTsSrc = path.join(__dirname, "services", "ExampleService.ts");
        const exampleProtoDest = path.join(protoDir, "ExampleService.proto");
        const exampleTsDest = path.join(servicesDir, "ExampleService.ts");
        const ExampleServiceProto = await fs.readFile(exampleProtoSrc, "utf8");
        const ExampleServiceTs = [
            `import { ServiceClient, service } from "ngrpc";`
        ].concat((await fs.readFile(exampleTsSrc, "utf8")).split("\n").slice(2)).join("\n");

        if (!(await exists(exampleProtoDest))) {
            await fs.writeFile(exampleProtoDest, ExampleServiceProto, "utf8");
            console.info(`example .proto file '${exampleProtoDest}' created`);
        }

        if (!(await exists(exampleTsDest))) {
            await fs.writeFile(exampleTsDest, ExampleServiceTs, "utf8");
            console.info(`example .ts file '${exampleTsDest}' created`);
        }
    });

async function handleStart(appName: string | undefined) {
    const conf = await ngrpc.loadConfig();

    const start = async (app: Config["apps"][0]) => {
        try {
            await spawnProcess(app, conf.entry);
            console.info(`app [${app.name}] started at '${app.uri}'`);
        } catch (err) {
            const reason = err.message || String(err);
            console.error(`unable to start app [${app.name}] (reason: ${reason})`);
        }
    };

    if (appName) {
        const app = conf.apps?.find(app => app.name === appName);

        if (!app) {
            throw new Error(`app [${appName}] doesn't exist in the config file`);
        } else if (!app.serve) {
            throw new Error(`app [${appName}] is not intended to be served`);
        }

        await start(app);
    } else {
        await Promise.all(conf.apps.filter(app => app.serve).map(start));
    }

    process.exit(0);
}

program.command("start")
    .description("start an app or all apps (exclude non-served ones)")
    .argument("[app]", "the app name in the config file")
    .action(handleStart);

program.command("restart")
    .description("restart an app or all apps (exclude non-served ones)")
    .argument("[app]", "the app name in the config file")
    .action(async (appName: string | undefined) => {
        try {
            await ngrpc.sendCommand("stop", appName);
            await handleStart(appName);
        } catch (err) {
            console.error(err.message || String(err));
        }
    });

program.command("reload")
    .description("reload an app or all apps")
    .argument("[app]", "the app name in the config file")
    .action(async (appName: string | undefined) => {
        try {
            await ngrpc.sendCommand("reload", appName);
        } catch (err) {
            console.error(err.message || String(err));
        }
    });

program.command("stop")
    .description("stop an app or all apps")
    .argument("[app]", "the app name in the config file")
    .action(async (appName: string | undefined) => {
        try {
            await ngrpc.sendCommand("stop", appName);
        } catch (err) {
            console.error(err.message || String(err));
        }
    });

program.command("list")
    .description("list all apps (exclude non-served ones)")
    .action(async () => {
        try {
            await ngrpc.sendCommand("list");
        } catch (err) {
            console.error(err);
        }
    });

if (process.send) {
    if (require.main?.filename === __filename) {
        const appName = process.argv[2];

        ngrpc.boot(appName).then(() => {
            process.send?.("ready");
        }).catch((err) => {
            console.error(err);
            process.exit(1);
        });
    }
} else {
    program.parse();
}
