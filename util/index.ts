import * as fs from "fs";
import * as path from "path";

// @ts-ignore
export const isTsNode = !!process[Symbol.for("ts-node.register.instance")];
export const sServiceName = Symbol.for("serviceName");

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
        !/^\\\\[.?]\\pipe\\/.test(filename)
    ) {
        filename = "\\\\.\\pipe\\" + filename;
    }

    return filename;
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
export function service(name: string): <T extends abstract new (...args: any[]) => any>(
    target: T,
    ctx?: any
) => void | T {
    return (target: any) => {
        target[sServiceName] = name;
        return target;
    };
}
