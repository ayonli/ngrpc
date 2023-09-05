import ngrpc from "@ayonli/ngrpc";

ngrpc.runSnippet(async () => {
    const userId = "ayon.li";

    const user = await services.UserService.getUser({ id: userId });
    console.log(user);

    const posts = await services.UserService.getMyPosts({ id: userId });
    console.log(posts);

    const reply = await services.ExampleService.sayHello({ name: "World" });
    console.log(reply.message);

    process.exit(0); // do not wait for idle
});
