#!/usr/bin/env node
import * as http from "http";
import { https } from "follow-redirects";
import * as path from "path";
import * as fs from "fs/promises";
import * as unzip from "unzipper";
import { exists, isTsNode } from "../util";
import { spawnSync } from "child_process";
import sleep from "@hyurl/utils/sleep";

const nodeModulesDir = path.dirname(path.dirname(path.dirname(__dirname)));
const hiddenDir = path.join(nodeModulesDir, ".ngrpc");
const cmdPath = path.join(hiddenDir, "ngrpc");
let zipName: string | undefined;

if (process.platform === "darwin") {
    if (process.arch === "arm64") {
        zipName = `ngrpc-mac-arm64.zip`;
    } else if (process.arch === "x64") {
        zipName = "ngrpc-mac-amd64.zip";
    }
} else if (process.platform === "linux") {
    if (process.arch === "arm64") {
        zipName = `ngrpc-linux-arm64.zip`;
    } else if (process.arch === "x64") {
        zipName = "ngrpc-linux-amd64.zip";
    }
} else if (process.platform === "win32") {
    if (process.arch === "arm64") {
        zipName = `ngrpc-linux-arm64.zip`;
    } else if (process.arch === "x64") {
        zipName = "ngrpc-linux-amd64.zip";
    }
}

async function ensureDir(dir: string) {
    try {
        await fs.mkdir(dir, { recursive: true });
    } catch { }
}

function reportImportFailure(err?: Error) {
    if (err) {
        console.error(err);
        console.error("");
    }

    console.error("cannot import ngrpc executable, try install it via:");
    console.error("    go install github.com/ayonli/ngrpc/cli/ngrpc@latest");
    process.exit(1);
}

(async function main() {
    if (!(await exists(cmdPath))) {
        if (!zipName) {
            reportImportFailure();
        }

        await ensureDir(hiddenDir);
        const pkg = isTsNode ? await import("../package.json") : require("../../package.json");
        const url = `https://github.com/ayonli/ngrpc/releases/download/v${pkg.version}/${zipName}`;
        const res = await new Promise<http.IncomingMessage>((resolve, reject) => {
            https.get(url, res => {
                resolve(res);
            }).once("error", reject);
        });

        if (res.statusCode !== 200) {
            reportImportFailure(new Error(`unable to download ${zipName}`));
        }

        res.pipe(unzip.Extract({ path: hiddenDir }));

        await new Promise<void>((resolve, reject) => {
            res.on("error", (err) => {
                reject(err);
            }).on("end", () => {
                resolve();
            });
        });

        if (process.platform !== "win32") {
            spawnSync("chmod", ["+x", cmdPath], { stdio: "inherit" });
            await sleep(100); // need to wait a while, don't know why
        }

        spawnSync(cmdPath, process.argv.slice(2), { stdio: "inherit" });
    } else {
        spawnSync(cmdPath, process.argv.slice(2), { stdio: "inherit" });
    }
})().catch(err => {
    reportImportFailure(err);
});
