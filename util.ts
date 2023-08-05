import * as fs from "fs";
import * as path from "path";
import { fork, ForkOptions } from "child_process";
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

export async function forkServer(app: Config["apps"][0], config = "") {
    const forkOptions: ForkOptions = {
        detached: true,
        silent: true,
    };
    let stdout: number;
    let stderr: number;

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
        forkOptions.stdio = ["ignore", stdout, stderr, "ipc"];
    } else {
        throw new Error("'stdout' must be configured in detach mode");
    }

    if (app.env) {
        forkOptions.env = app.env;
    }

    const child = fork(
        app.entry || path.join(__dirname, "cli"),
        config ? [app.name, config] : [app.name],
        forkOptions);

    await new Promise<void>((resolve, reject) => {
        child.on("disconnect", () => {
            resolve();
        }).on("error", err => {
            reject(err);
        });
    });
    child.unref();
}
