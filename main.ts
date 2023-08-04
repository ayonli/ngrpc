import App from ".";

if (require.main?.filename === __filename) {
    const appName = process.argv[2];
    const config = process.argv[3];

    App.boot(appName, config).catch(console.error).finally(() => {
        process.disconnect();
    });
}
