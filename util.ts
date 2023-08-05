import * as fs from "fs";
import * as path from "path";
import { spawn, SpawnOptions } from "child_process";
import { Config } from ".";

export async function ensureDir(dirname: string) {
    try {
        await fs.promises.mkdir(dirname, { recursive: true });
    } catch (err) {
        if (err["code"] !== "EEXIST")
            throw err;
    }
}

export function absPath(filename: string, withPipe = false): string {
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

export async function spawnProcess(app: Config["apps"][0], config = "") {
    let entry = app.entry || path.join(__dirname, "cli");
    const ext = path.extname(entry).toLowerCase();
    let execCmd: "node" | "ts-node";
    let stdout: number;
    let stderr: number;
    const options: SpawnOptions = {
        detached: true,
    };

    if (ext === ".js") {
        execCmd = "node";
    } else if (ext === ".ts") {
        execCmd = "ts-node";
    } else if (fs.existsSync(entry + ".js")) {
        execCmd = "node";
    } else if (fs.existsSync(entry + ".ts")) {
        execCmd = "ts-node";
    } else {
        throw new Error("Cannot determine the type of the entry file");
    }

    if (app.stdout) {
        const filename = absPath(app.stdout);
        await ensureDir(path.dirname(filename));
        stdout = fs.openSync(filename, "a");
    }

    if (app.stderr) {
        const filename = absPath(app.stderr);
        await ensureDir(path.dirname(filename));
        stderr = fs.openSync(filename, "a");
    } else if (stdout) {
        stderr = fs.openSync(absPath(app.stdout), "a");
    }

    if (stdout && stderr) {
        options.stdio = ["ignore", stdout, stderr, "ipc"];
    } else {
        options.stdio = ["ignore", "inherit", "inherit", "ipc"];
    }

    if (app.env) {
        options.env = app.env;
    }

    const args: string[] = [entry, app.name];

    if (config) {
        args.push(config);
    }

    if (execCmd === "ts-node") {
        args.unshift("-r", "ts-node/register");
    }

    const child = spawn("node", args, options);
    await new Promise<void>((resolve, reject) => {
        child.on("disconnect", () => {
            resolve();
        }).on("message", (msg) => {
            if (msg === "ready") {
                resolve();
            }
        }).on("error", err => {
            reject(err);
        });
    });
    child.unref();
}
