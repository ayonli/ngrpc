import * as fs from "fs";
import * as path from "path";

export const isTsNode = !!process[Symbol.for("ts-node.register.instance")];
export const sServiceName = Symbol.for("serviceName");

export type CpuUsage = NodeJS.CpuUsage & {
    uptime: number;
    percent: number;
    readonly _start?: { cpuUsage: NodeJS.CpuUsage; time: number; };
};

export async function exists(filename: string) {
    try {
        await fs.promises.stat(filename);
        return true;
    } catch {
        return false;
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

export function timed(callSite: TemplateStringsArray, ...bindings: any[]) {
    const text = callSite.map((str, i) => {
        return i > 0 ? bindings[i - 1] + str : str;
    }).join("");
    const now = new Date();
    const year = now.getFullYear();
    const month = String(now.getMonth() + 1).padStart(2, "0");
    const date = String(now.getDate()).padStart(2, "0");
    const hours = String(now.getHours()).padStart(2, "0");
    const minutes = String(now.getMinutes()).padStart(2, "0");
    const seconds = String(now.getSeconds()).padStart(2, "0");

    return `${year}/${month}/${date} ${hours}:${minutes}:${seconds} ${text}`;
}

/**
 * This decorator function is used to link the service class to a gRPC service.
 * 
 * @param name The service name defined in the `.proto` file.
 */
export function service(name: string): <T extends new (...args: any[]) => any>(
    target: T,
    ctx?: any
) => void | T {
    return (target) => {
        target[sServiceName] = name;
        return target;
    };
}
