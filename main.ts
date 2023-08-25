import App from "."; // replace `.` with `@hyurl/grpc-boot`

if (require.main?.filename === __filename) {
    const appName = process.argv[2];

    App.boot(appName).then(() => {
        process.send?.("ready");
    }).catch(err => {
        console.error(err);
        process.exit(1);
    });
}
