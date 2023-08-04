/// <reference path="./services/PostService.ts" />
/// <reference path="./services/UserService.d.ts" />
import { App } from ".";

if (require.main?.filename === __filename) {
    const appName = process.argv[2];
    const config = process.argv[3];
    const app = new App(config);

    app.start(appName).catch(console.error).finally(() => {
        process.disconnect();
    });
}
