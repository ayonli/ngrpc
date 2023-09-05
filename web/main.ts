import * as http from "http";
import * as https from "https";
import * as express from "express";
import ngrpc from "@ayonli/ngrpc";
import _try from "dotry";
import { Post, User } from "../services/struct";

type ApiResponse<T> = {
    code: number;
    data?: T;
    error?: string;
};

(async () => {
    const appName = ngrpc.getAppName();
    const app = await ngrpc.start(appName);
    app.waitForExit();

    if (app.name !== "web-server") {
        process.send?.("ready"); // for PM2 compatibility
        return;
    }

    let httpServer: http.Server;
    let httpsServer: https.Server;
    const route = express();

    const startWebServer = () => {
        const { protocol, port } = new URL(app.uri);

        if (protocol === "https:") {
            httpsServer = https.createServer(route).listen(port || "443", () => {
                process.send?.("ready");
            });
        } else {
            httpServer = http.createServer(route).listen(port || "80", () => {
                process.send?.("ready");
            });
        }
    };

    startWebServer();

    app.onStop(() => {
        httpServer?.close();
        httpsServer?.close();
    });
    app.onReload(() => {
        // restart the web server with the newest configuration.
        httpServer?.close(startWebServer);
        httpsServer?.close(startWebServer);
    });

    route.get("/user/:id", async (req, res) => {
        type UserResponse = ApiResponse<User>;
        const userId = req.params.id;
        const [err, user] = await _try(services.UserService.getUser({ id: userId }));

        if (err) {
            if (err.message.includes("not found")) {
                res.json({ code: 404, error: err.message } satisfies UserResponse);
            } else {
                res.json({ code: 500, error: err.message } satisfies UserResponse);
            }
        } else {
            res.json({ code: 0, data: user } satisfies UserResponse);
        }
    }).get("/users/gender/:gender", async (req, res) => {
        type UsersResponse = ApiResponse<User[]>;
        let gender: 0 | 1 | 2;

        if (req.params.gender === "unknown") {
            gender = 0;
        } else if (req.params.gender === "male") {
            gender = 1;
        } else if (req.params.gender === "female") {
            gender = 2;
        } else {
            res.json({
                code: 400,
                error: "unrecognized gender argument, shall be either 'male', 'female' or 'unknown'",
            } satisfies UsersResponse);
            return;
        }

        const [err, result] = await _try(services.UserService.getUsers({ gender: gender }));

        if (err) {
            res.json({ code: 500, error: err.message } satisfies UsersResponse);
        } else {
            res.json({ code: 0, data: result.users } satisfies UsersResponse);
        }
    }).get("/users/age/:min-:max", async (req, res) => {
        type UsersResponse = ApiResponse<User[]>;
        const minAge = parseInt(req.params.min);
        const maxAge = parseInt(req.params.max);

        if (isNaN(minAge) || isNaN(maxAge)) {
            res.json({ code: 400, error: "unrecognized age range" } satisfies UsersResponse);
            return;
        }

        const [err, result] = await _try(services.UserService.getUsers({ minAge, maxAge }));

        if (err) {
            res.json({ code: 500, error: err.message } satisfies UsersResponse);
        } else {
            res.json({ code: 0, data: result.users } satisfies UsersResponse);
        }
    }).get("/user/:id/posts", async (req, res) => {
        type PostsResponse = ApiResponse<Post[]>;
        const userId = req.params.id;
        const [err, result] = await _try(services.UserService.getMyPosts({ id: userId }));

        if (err) {
            res.json({ code: 500, error: err.message } satisfies PostsResponse);
        } else {
            res.json({ code: 0, data: result.posts } satisfies PostsResponse);
        }
    }).get("/post/:id", async (req, res) => {
        type PostResponse = ApiResponse<Post>;
        const id = parseInt(req.params.id);

        if (isNaN(id)) {
            res.json({ code: 400, error: "invalid post id" } satisfies PostResponse);
            return;
        }

        const [err, post] = await _try(services.PostService.getPost({ id }));

        if (err) {
            if (err.message.includes("not found")) {
                res.json({ code: 404, error: err.message } satisfies PostResponse);
            } else {
                res.json({ code: 500, error: err.message } satisfies PostResponse);
            }
        } else {
            res.json({ code: 0, data: post } satisfies PostResponse);
        }
    }).get("/posts/search/:keywords", async (req, res) => {
        type PostsResponse = ApiResponse<Post[]>;
        const [err, result] = await _try(services.PostService.searchPosts({
            keyword: req.params.keywords,
        }));

        if (err) {
            res.json({ code: 500, error: err.message } satisfies PostsResponse);
        } else {
            res.json({ code: 0, data: result.posts } satisfies PostsResponse);
        }
    });
})().catch(err => {
    console.error(err);
    process.exit(1);
});
