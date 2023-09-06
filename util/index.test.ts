import { test } from "mocha";
import * as assert from "assert";
import * as path from "path";
import { absPath, exists, service, timed } from ".";

test("exists", async () => {
    const ok1 = await exists("ngrpc.json");
    const ok2 = await exists("ngrpc.local.json");

    assert.ok(ok1);
    assert.ok(!ok2);
});

test("absPath", async () => {
    const file1 = absPath("./ngrpc.json");
    const file2 = absPath("/usr/local/bin");

    assert.strictEqual(file1, path.join(process.cwd(), "ngrpc.json"));
    assert.strictEqual(file2, "/usr/local/bin");

    if (process.platform === "win32") {
        const filename = "C:\\Program Files\\nodejs\\bin";
        const file3 = absPath(filename);
        assert.strictEqual(file3, filename);

        const file4 = absPath(filename, true);
        assert.strictEqual(file4, "\\\\.\\pipe\\" + filename);
    }
});

test("timed", () => {
    const str = timed`everything is fine`;
    assert.ok(str.match(/^\d{4}\/\d{2}\/\d{2} \d{2}:\d{2}:\d{2} /));
    assert.ok(str.endsWith(" everything is fine"));
});

test("@service", () => {
    class Foo { }
    class Bar { }

    service("services.Foo")(Foo);
    service("services.Bar")(Bar, {});

    // @ts-ignore
    assert.strictEqual(Foo[Symbol.for("serviceName")], "services.Foo");
    // @ts-ignore
    assert.strictEqual(Bar[Symbol.for("serviceName")], "services.Bar");
});
