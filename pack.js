const { spawnSync } = require("child_process");
const path = require("path");
const cfg = require("./package.json");
const pkg = "github.com/ayonli/ngrpc/cli/ngrpc";

/** @typedef {"linux" | "darwin" | "windows"} OS */
/** @typedef {"amd64" | "arm64"} Arch */

/**
 * @type {{ os: OS, arch: Arch[] }[]} targets
 */
const targets = [
    { os: "linux", arch: ["amd64", "arm64"] },
    { os: "darwin", arch: ["amd64", "arm64"] },
    { os: "windows", arch: ["amd64", "arm64"] }
];

for (const { os, arch } of targets) {
    console.log("packing for", os, "...");

    for (const _arch of arch) {
        const wd = path.join("prebuild", os, _arch);
        const exeName = os === "windows" ? "ngrpc.exe" : "ngrpc";
        const outPath = path.join(wd, exeName);
        spawnSync("go", [
            "build",
            "-o",
            outPath,
            `-ldflags`,
            `-X '${pkg}/cmd.version=v${cfg.version}'`,
            pkg
        ], {
            stdio: "inherit",
            env: {
                ...process.env,
                GOOS: os,
                GOARCH: _arch,
            }
        });
        spawnSync("tar", [
            "-czf",
            path.join("prebuild", `ngrpc-${os}-${_arch}.tgz`),
            "-C",
            wd,
            exeName
        ]);
    }
}
