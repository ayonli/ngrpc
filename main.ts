/// <reference path="./services/PostService.ts" />
/// <reference path="./services/UserService.d.ts" />
import { App } from ".";

if (require.main?.filename === __filename) {
    App.runSnippet(async () => {
        const post = await services.PostService.getPost({ id: 2 });
        console.log(post);
    });
}
