import "@ayonli/jsext/function";
import { test } from "mocha";
import * as assert from "assert";
import * as path from "path";
import * as fs from "fs";
import { App } from "../app";
import { exists } from "../util";
import { Guest, ControlMessage, encodeMessage, decodeMessage, getSocketPath } from "./guest";

function newMsg(msg: ControlMessage): ControlMessage {
    msg.app ??= "";
    msg.error ??= "";
    msg.fin ??= false;
    msg.msgId ??= "";
    msg.pid ??= 0;
    msg.guests ??= [];
    msg.text ??= "";

    return { ...msg };
}

test("encodeMessage", () => {
    const msg = encodeMessage(newMsg({ cmd: "stop", app: "example-server", msgId: "abc" }));
    assert.strictEqual(msg[msg.length - 1], "\n");
});

test("decodeMessage", () => {
    const msg: ControlMessage = newMsg({ cmd: "stop", app: "example-server", msgId: "abc" });
    const data = encodeMessage(msg);
    let packet = "";
    let buf = data.slice(0, 256);

    const { packet: _packet, messages } = decodeMessage(packet, buf, false);
    packet = _packet;

    assert.strictEqual(messages.length, 1);
    assert.deepStrictEqual(messages[0], msg);
    assert.strictEqual(packet, "");
});

test("decodeMessage overflow", () => {
    const msg: ControlMessage = newMsg({ cmd: "stop", app: "example-server", msgId: "abc" });
    const data = encodeMessage(msg);
    let packet = "";
    let buf = data.slice(0, 64);
    let offset = 0;

    let result = decodeMessage(packet, buf, false);
    let messages = result.messages;
    packet = result.packet;
    offset += 64;

    assert.strictEqual(messages.length, 0);
    assert.strictEqual(packet, buf);

    while (offset < data.length) {
        buf = data.slice(offset, offset + 64);
        const result = decodeMessage(packet, buf, false);
        messages = result.messages;
        packet = result.packet;
        offset += 64;
    }

    assert.strictEqual(messages.length, 1);
    assert.deepStrictEqual(messages[0], msg);
    assert.strictEqual(packet, "");
});

test("decodeMessage EOF", () => {
    const msg: ControlMessage = newMsg({ cmd: "stop", app: "example-server", msgId: "abc" });
    const data = encodeMessage(msg).slice(0, -1);
    let packet = "";
    let buf = data.slice(0, 256);

    let result = decodeMessage(packet, buf, true);
    let messages = result.messages;
    packet = result.packet;

    assert.strictEqual(messages.length, 1);
    assert.deepStrictEqual(messages[0], msg);
    assert.strictEqual(packet, "");
});

test("getSocketPath", () => {
    const cwd = process.cwd();
    const { sockFile, sockPath } = getSocketPath();

    assert.strictEqual(sockFile, path.join(cwd, "ngrpc.sock"));

    if (process.platform === "win32") {
        assert.strictEqual(sockPath, "\\\\.\\pipe\\" + path.join(cwd, "ngrpc.sock"));
    } else {
        assert.strictEqual(sockPath, path.join(cwd, "ngrpc.sock"));
    }
});

test("new Guest", () => {
    const app: App = {
        name: "example-server",
        url: "grpc://localhost:4000",
        services: [],
    };
    const handleStop = () => void 0;
    const handleReload = () => void 0;
    const guest = new Guest(app, { onStopCommand: handleStop, onReloadCommand: handleReload });

    assert.strictEqual(guest.appName, app.name);
    assert.strictEqual(guest.appUrl, app.url);
    assert.strictEqual(guest["handleStopCommand"], handleStop);
    assert.strictEqual(guest["handleReloadCommand"], handleReload);
});

test("Guest join redundant socket file", async () => {
    const { sockFile } = getSocketPath();
    fs.writeFileSync(sockFile, Buffer.from([]), "binary");

    assert.ok(await exists(sockFile));

    const guest = new Guest({
        name: "example-server",
        url: "grpc://localhost:4000",
        services: [],
    }, {
        onStopCommand: () => void 0,
        onReloadCommand: () => void 0,
    });
    const [err] = await Function.try(() => guest["connect"]());

    assert.ok(!!err);
    assert.ok(!(await exists(sockFile)));
});
