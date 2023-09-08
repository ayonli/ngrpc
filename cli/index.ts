#!/usr/bin/env node
import * as http from "http";
import { https } from "follow-redirects";
import * as path from "path";
import * as fs from "fs/promises";
import * as tar from "tar";
import { exists, isTsNode } from "../util";
import { spawnSync } from "child_process";

const exePath = path.join(__dirname, process.platform === "win32" ? "ngrpc.exe" : "ngrpc");
const os = process.platform === "win32" ? "windows" : process.platform;
const zipName = `ngrpc-${os}-${process.arch}.tgz`;

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
    if (!(await exists(exePath))) {
        if (!zipName) {
            reportImportFailure();
        }

        const pkg = isTsNode ? require("../package.json") : require("../../package.json");
        const url = `https://github.com/ayonli/ngrpc/releases/download/v${pkg.version}/${zipName}`;
        const res = await new Promise<http.IncomingMessage>((resolve, reject) => {
            https.get(url, res => {
                resolve(res);
            }).once("error", reject);
        });

        if (res.statusCode !== 200) {
            reportImportFailure(new Error(`unable to download ${zipName}`));
        }

        await new Promise<void>((resolve, reject) => {
            const out = tar.extract({ cwd: __dirname });
            const handleError = async (err: Error) => {
                try { await fs.unlink(exePath); } catch { }
                reject(err);
            };

            res.pipe(out);
            res.on("error", handleError);
            out.on("error", handleError).on("finish", resolve);
        });

        spawnSync(exePath, process.argv.slice(2), { stdio: "inherit" });
    } else {
        spawnSync(exePath, process.argv.slice(2), { stdio: "inherit" });
    }
})().catch(err => {
    reportImportFailure(err);
});
