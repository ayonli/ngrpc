#!/usr/bin/env node
import "source-map-support/register";
import * as commander from "commander";
import * as fs from "fs";
import * as path from "path";
import { fork } from "child_process";
import pkg = require("./package.json");
import { App, Config } from ".";
import { absPath, ensureDir } from "./util";

const program = new commander.Command("grpc-boot");

program.description("start, reload or stop gRPC apps")
    .version(pkg.version);

program.command("init")
    .description("initiate a new gRPC project")
    .argument("[package]", "The package name / root namespace of the services, default 'services'")
    .option("-c, --config <filename>", "create a custom config file")
    .action(async (pkg: string, options) => {
        pkg ||= "services";
        const tsConfig = absPath("tsconfig.json");
        const config = absPath(options.config || "boot.config.json");
        const dir = absPath(pkg);

        if (fs.existsSync(tsConfig)) {
            console.warn(`File '${path.basename(config)}' already exists`);
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

            fs.writeFileSync(tsConfig, JSON.stringify(tsConf, null, "    "), "utf8");
            console.info(`TSConfig file written to '${path.basename(tsConfig)}'`);
        }

        if (fs.existsSync(config)) {
            console.warn(`File '${path.basename(config)}' already exists`);
        } else {
            const conf: Config = {
                "$schema": "./node_modules/@hyurl/grpc-boot/boot.config.schema.json",
                "package": pkg,
                "protoDirs": ["./" + pkg],
                "protoOptions": {
                    "keepCase": true,
                    // @ts-ignore
                    "longs": "String",
                    // @ts-ignore
                    "enums": "String",
                    "defaults": true,
                    "oneofs": true
                },
                "apps": [
                    {
                        "name": "example-server",
                        "uri": "grpc://localhost:4000",
                        "serve": true,
                        "services": [
                            `${pkg}.ExampleService`
                        ]
                    }
                ]
            };
            fs.writeFileSync(config, JSON.stringify(conf, null, "    "), "utf8");
            console.info(`Config file written to '${path.basename(config)}'`);
        }

        if (fs.existsSync(dir)) {
            console.warn(`Path '${dir}' already exists`);
        } else {
            await ensureDir(dir);
            console.info(`Path '${dir}' created`);
        }

        const exampleProtoSrc = path.join(__dirname, "services", "ExampleService.proto");
        const exampleTsSrc = path.join(__dirname, "services", "ExampleService.ts");
        const exampleProtoDest = absPath(pkg + path.sep + "ExampleService.proto");
        const exampleTsDest = absPath(pkg + path.sep + "ExampleService.ts");
        const ExampleServiceProto = fs.readFileSync(exampleProtoSrc, "utf8")
            .replace("package services;", `package ${pkg};`);
        const ExampleServiceTs = fs.readFileSync(exampleTsSrc, "utf8")
            .replace("namespace services {", `namespace ${pkg} {`);

        if (!fs.existsSync(exampleProtoDest)) {
            fs.writeFileSync(exampleProtoDest, ExampleServiceProto, "utf8");
            console.info(`Example .proto file '${exampleProtoDest}' created`);
        }

        if (!fs.existsSync(exampleTsDest)) {
            fs.writeFileSync(exampleTsDest, ExampleServiceTs, "utf8");
            console.info(`Example .ts file '${exampleTsDest}' created`);
        }
    });

program.command("start")
    .description("start a gRPC app")
    .argument("<app>", "the app name in the config file")
    .option("-d, --detach", "allow the CLI command to exit after starting the app")
    .option("-c, --config <filename>", "use a custom config file")
    .action(async (appName: string, options) => {
        if (options.detach) { // fork a child process to start the app
            const conf = App.loadConfig(options.config);
            const app = conf.apps?.find(app => app.name === appName);

            if (!app) {
                throw new Error(`gRPC app [${appName}] doesn't exist in the config file`);
            } else if (!app.serve) {
                throw new Error(`gRPC app [${appName}] is not intended to be served`);
            }

            const child = fork(__filename, options.config ? [appName, options.config] : [appName], {
                detached: true,
                silent: true,
            });

            await new Promise<void>((resolve, reject) => {
                child.on("disconnect", () => {
                    resolve();
                }).on("error", err => {
                    reject(err);
                });
            });

            console.info(`gRPC app [${appName}] started at '${app.uri}'`);

            process.exit(0);
        } else {
            try {
                const app = new App(options.config);
                await app.start(appName);
            } catch (err) {
                console.error(err.message || String(err));
            }
        }
    });

program.command("reload")
    .description("reload a gRPC app or all gRPC apps")
    .argument("[app]", "the app name in the config file")
    .option("-c, --config <filename>", "use a custom config file")
    .action(async (appName: string | undefined, options) => {
        try {
            await App.sendCommand("reload", appName, options.config);
        } catch (err) {
            console.error(err.message || String(err));
        }
    });

program.command("stop")
    .description("stop a gRPC or all gRPC apps")
    .argument("[app]", "the app name in the config file")
    .option("-c, --config <filename>", "use a custom config file")
    .action(async (appName: string | undefined, options) => {
        try {
            await App.sendCommand("stop", appName, options.config);
        } catch (err) {
            console.error(err.message || String(err));
        }
    });

program.command("list")
    .description("list all gRPC apps (exclude pure-clients apps)")
    .option("c, --config <filename>", "use a custom config file")
    .action(async (_, options) => {
        try {
            await App.sendCommand("list", void 0, options.config);
        } catch (err) {
            console.error(err);
        }
    });

if (process.send) {
    const appName = process.argv[2];
    const config = process.argv[3];

    if (appName) {
        const app = new App(config);
        app.start(appName).catch(console.error).finally(() => {
            process.disconnect();
        });
    }
} else {
    program.parse();
}
