import * as fs from "fs/promises";
import * as path from "path";

export async function ensureDir(dirname: string) {
    try {
        await fs.mkdir(dirname, { recursive: true });
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
