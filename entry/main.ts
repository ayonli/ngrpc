import ngrpc from ".."; // replace `.` with `@ayonli/ngrpc`

if (require.main?.filename === __filename) {
    ngrpc.start(ngrpc.getAppName()).then(app => {
        process.send?.("ready"); // for PM2 compatibility
        app.waitForExit();
    }).catch(err => {
        console.error(err);
        process.exit(1);
    });
}
