import * as path from "node:path";
import * as net from "node:net";
import { exists, remove } from "@ayonli/jsext/fs";
import { absPath, timed } from "../util";
import type { App } from "../app";

export interface ControlMessage {
    cmd: "handshake" | "goodbye" | "reply" | "stop" | "reload";
    app?: string;
    msgId?: string;
    text?: string;
    guests?: {
        app: string;
        pid: number;
        startTime: number;
    }[];
    error?: string;

    // `pid` shall be provided when `cmd` is `handshake`.
    pid?: number;

    // Indicates that this is the last message, after set true, the socket connection will be closed
    // by the receiver peer.
    fin?: boolean;
}

export function encodeMessage(msg: ControlMessage): string {
    return JSON.stringify(msg) + "\n";
}

export function decodeMessage(packet: string, buf: string, eof = false): {
    packet: string;
    messages: ControlMessage[];
} {
    packet += buf;
    let chunks = packet.split("\n");

    if (eof) {
        // Empty the packet when reaching EOF.
        packet = "";
        // Returns all non-empty chunks, normally the last chunk is empty.
        chunks = chunks.filter(chunk => chunk.length > 0);
    } else if (chunks.length > 1) {
        // The last chunk is unfinished, we store it in the packet for more data.
        packet = chunks[chunks.length - 1] as string;
        // All chunks (except the last one) will be processed.
        chunks = chunks.slice(0, -1);
    } else { // chunks.length === 1
        // We use `\n` to delimit message packets, each packet ends with a `\n`, when len(chunks)
        // is 1, it means that the delimiter haven't been received and there is more buffers needs
        // to be received, no available chunks for consuming yet.
        return { packet, messages: [] };
    }

    const messages: ControlMessage[] = [];

    for (const chunk of chunks) {
        try {
            const msg = JSON.parse(chunk);
            messages.push(msg);
        } catch { }
    }

    return { packet, messages };
}

export function getSocketPath() {
    const confFile = absPath("ngrpc.json");
    const ext = path.extname(confFile);
    const sockFile = confFile.slice(0, -ext.length) + ".sock";
    const sockPath = absPath(sockFile, true);

    return { sockFile, sockPath };
}

export class Guest {
    appName: string;
    appUrl: string;
    /** 0: disconnected; 1: connected; 2: closed */
    private state = 0;
    private conn: net.Socket | undefined;
    private reconnector: NodeJS.Timeout | null = null;
    private handleStopCommand: (msgId: string | undefined) => void;
    private handleReloadCommand: (msgId: string | undefined) => void;

    constructor(app: App, options: {
        onStopCommand: (msgId: string | undefined) => void;
        onReloadCommand: (msgId: string | undefined) => void;
    }) {
        this.appName = app.name;
        this.appUrl = app.url;
        this.handleStopCommand = options?.onStopCommand;
        this.handleReloadCommand = options?.onReloadCommand;
    }

    async join() {
        try {
            await this.connect();
        } catch {
            this.reconnect(); // auto-reconnect in the background
        }
    }

    private async connect(): Promise<void> {
        const { sockFile, sockPath } = getSocketPath();

        if (process.platform !== "win32" && !(await exists(sockFile))) {
            throw new Error("host server is not running");
        }

        await new Promise<void>(async (handshake, reject) => {
            const connectFailureHandler = async (err: Error) => {
                try { await remove(sockFile); } catch { }
                reject(err);
            };

            const conn = net.createConnection(sockPath, () => {
                this.conn = conn;
                this.send({ cmd: "handshake", app: this.appName, pid: process.pid });
                conn.off("error", connectFailureHandler);

                (async () => {
                    let packet = "";

                    try {
                        for await (const buf of conn) {
                            packet = this.processHostMessage(
                                handshake,
                                packet,
                                (buf as Buffer).toString());
                        }
                    } catch (err) {
                        if (this.conn?.destroyed || this.conn?.closed) {
                            this.handleHostDisconnection();
                        } else {
                            console.error(timed`${err}`);
                        }
                    }
                })();
            });
            conn.once("error", connectFailureHandler);
        });

        if (this.appName) {
            console.log(timed`app [${this.appName}] has joined the group`);
        }
    }

    async leave(reason: string, replyId = ""): Promise<boolean> {
        if (this.state !== 0) {
            if (replyId) {
                // If `replyId` is provided, that means the stop event is issued by a guest app, for
                // example, the CLI tool, in this case, we need to send feedback to acknowledge the
                // sender that the process has finished.
                //
                // Apparently there is some compatibility issues in the Golang's go-winio package,
                // if we sent the messages one by one continuously, go-winio cannot receive them
                // well. So we send them in one packet, allowing the host server to separate them
                // when received as a whole.
                this.send({ cmd: "goodbye", app: this.appName }, {
                    cmd: "reply",
                    app: this.appName,
                    msgId: replyId,
                    text: reason,
                    fin: true,
                });
            } else {
                this.send({ cmd: "goodbye", app: this.appName, fin: true });
            }
        } else if (this.reconnector) {
            clearInterval(this.reconnector);
            this.reconnector = null;
        }

        const ok = this.state == 1;
        this.state = 2;
        return ok;
    }

    async send(...msg: ControlMessage[]) {
        this.conn?.write(msg.map(m => encodeMessage(m)).join(""));
    }

    private async reconnect() {
        this.reconnector = setInterval(async () => {
            if (this.state === 2) {
                this.reconnector && clearInterval(this.reconnector);
                this.reconnector = null;
            } else {
                try {
                    await this.connect();

                    if (this.state !== 0) {
                        this.reconnector && clearInterval(this.reconnector);
                        this.reconnector = null;
                    }
                } catch { }
            }
        }, 1_000);
    }

    private handleHostDisconnection() {
        if (this.state === 0) {
            return;
        } else if (this.state !== 2) {
            this.state = 0;
            this.reconnect();
        }
    }

    private processHostMessage(handshake: () => void, packet: string, buf: string) {
        const res = decodeMessage(packet, buf, false);

        for (const msg of res.messages) {
            this.handleMessage(handshake, msg);
        }

        return packet;
    }

    private handleMessage(handshake: () => void, msg: ControlMessage) {
        if (msg.cmd === "handshake") {
            this.state = 1;
            handshake();
        } else if (msg.cmd === "goodbye") {
            this.conn?.destroy();
        } else if (msg.cmd === "stop") {
            this.handleStopCommand(msg.msgId);
        } else if (msg.cmd === "reload") {
            this.handleReloadCommand(msg.msgId);
        }
    }
}
