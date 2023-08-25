import * as fs from "fs";
import * as path from "path";
import { promisify } from "util";
import { spawn, SpawnOptions } from "child_process";
import { Config } from "./app";

export const isTsNode = !!process[Symbol.for("ts-node.register.instance")];
export const sServiceName = Symbol.for("serviceName");

export type CpuUsage = NodeJS.CpuUsage & {
    uptime: number;
    percent: number;
    readonly _start?: { cpuUsage: NodeJS.CpuUsage; time: number; };
};

export const open = promisify(fs.open);

export async function exists(filename: string) {
    try {
        await fs.promises.stat(filename);
        return true;
    } catch {
        return false;
    }
}

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

export async function spawnProcess(app: Config["apps"][0], defaultEntry = "") {
    const entry = app.entry || defaultEntry || path.join(__dirname, "cli");
    const ext = path.extname(entry).toLowerCase();
    let execCmd: "node" | "ts-node";
    let stdout: number | undefined;
    let stderr: number | undefined;
    const options: SpawnOptions = {
        detached: true,
    };

    if (ext === ".js") {
        execCmd = "node";
    } else if (ext === ".ts") {
        execCmd = "ts-node";
    } else if (await exists(entry + ".js")) {
        execCmd = "node";
    } else if (await exists(entry + ".ts")) {
        execCmd = "ts-node";
    } else {
        throw new Error("Cannot determine the type of the entry file");
    }

    if (app.stdout) {
        const filename = absPath(app.stdout);
        await ensureDir(path.dirname(filename));
        stdout = await open(filename, "a");
    }

    if (app.stderr) {
        const filename = absPath(app.stderr);
        await ensureDir(path.dirname(filename));
        stderr = await open(filename, "a");
    } else if (app.stdout) {
        stderr = await open(absPath(app.stdout), "a");
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

    if (execCmd === "ts-node") {
        args.unshift("-r", "ts-node/register");
    }

    const child = spawn("node", args, options);
    await new Promise<void>((resolve, reject) => {
        child.on("message", (msg) => {
            if (msg === "ready") {
                resolve();
            }
        }).on("error", err => {
            reject(err);
        }).on("exit", (code) => {
            reject(new Error(`Child process exited unexpectedly (code: ${code})`));
        });
    });
}

export function getCpuUsage(oldUsage: CpuUsage | null = null) {
    let usage: CpuUsage;

    if (oldUsage?._start) {
        usage = {
            ...process.cpuUsage(oldUsage._start.cpuUsage),
            uptime: Date.now() - oldUsage._start.time,
            percent: 0,
        };
    } else {
        usage = {
            ...process.cpuUsage(),
            uptime: process.uptime() * 1000,
            percent: 0
        };
    }

    usage.percent = (usage.system + usage.user) / (usage.uptime * 10);

    Object.defineProperty(usage, "_start", {
        value: {
            cpuUsage: process.cpuUsage(),
            time: Date.now(),
        }
    });

    return usage as CpuUsage;
}

/**
 * This decorator function is used to link the service class to a gRPC service.
 * 
 * @param name The service name defined in the `.proto` file.
 */
export function service(name: string): (target: Function, ctx: ClassDecoratorContext) => void;
export function service(name: string, ): ClassDecorator;
export function service(name: string): ClassDecorator {
    return (target) => {
        target[Symbol.for("serviceName")] = name;
        return target;
    };
}
