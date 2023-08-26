/// <reference path="./services/PostService.ts" />
import ngrpc from "."; // replace `.` with `ngrpc`

if (require.main?.filename === __filename) {
    ngrpc.runSnippet(async () => {
        const post = await services.PostService.getPost({ id: 1 });
        console.log(post);
    });
}
