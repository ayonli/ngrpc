import ngrpc from ".."; // replace `.` with `@ayonli/ngrpc`

if (require.main?.filename === __filename) {
    const appName = process.argv[2];

    ngrpc.boot(appName).then(app => {
        process.send?.("ready"); // for PM2
        app.waitForExit();
    }).catch(err => {
        console.error(err);
        process.exit(1);
    });
}
