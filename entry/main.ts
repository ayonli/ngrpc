import ngrpc from ".."; // replace `.` with `@ayonli/ngrpc`

if (require.main?.filename === __filename) {
    const appName = process.argv[2];

    ngrpc.boot(appName).then(() => {
        process.send?.("ready");
    }).catch(err => {
        console.error(err);
        process.exit(1);
    });
}
