/// <reference path="./services/PostService.ts" />
/// <reference path="./services/UserService.d.ts" />
import { BootApp } from ".";

if (require.main?.filename === __filename) {
    (async () => {
        const app = new BootApp();

        await app.start("post-server");

        try {
            const post = await services.PostService.getPost({ id: 2 });
            console.log(post);
        } catch (err) {
            console.error(err);
        }

        BootApp.issue("stop");
    })();
}
