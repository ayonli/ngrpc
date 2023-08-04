/// <reference path="./services/PostService.ts" />
import App from ".";

if (require.main?.filename === __filename) {
    App.runSnippet(async () => {
        const post = await services.PostService.getPost({ id: 1 });
        console.log(post);
    });
}
