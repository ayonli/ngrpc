/// <reference path="./UserService.d.ts" />
import _try from "dotry";
import { ServiceClient } from "@hyurl/grpc-async";
import { Post } from "./struct";

declare global {
    namespace services {
        const PostService: ServiceClient<PostService>;
    }
}

export type PostQuery = {
    id: number;
};

export type PostsQuery = {
    author?: string;
    keyword?: string;
};

export type PostSearchResult = {
    posts: Post[];
};

export default class PostService {
    private postStore: (Omit<Post, "author"> & { author: string; })[] = [
        {
            id: 1,
            title: "My first article",
            description: "This is my first article",
            content: "The article contents ...",
            author: "ayon.li",
        },
        {
            id: 2,
            title: "My second article",
            description: "This is my second article",
            content: "The article content ...",
            author: "ayon.li",
        }
    ];

    async getPost(query: PostQuery): Promise<Post> {
        const post = this.postStore.find(item => item.id === query.id);

        if (post) {
            const [err, author] = await _try(() => services.UserService.getUser({ id: post.author, }));

            if (!err && author) {
                return { ...post, author };
            } else {
                return { ...post, author: null };
            }
        } else {
            throw new Error(`Post ${query.id} not found`);
        }
    }

    async searchPosts(query: PostsQuery): Promise<PostSearchResult> {
        if (query.author) {
            const _posts = this.postStore.filter(item => item.author === query.author);

            if (_posts.length) {
                const [err, author] = await _try(services.UserService.getUser({ id: query.author, }));

                if (!err && author) {
                    return { posts: _posts.map(post => ({ ...post, author })) };
                } else {
                    return { posts: _posts.map(post => ({ ...post, author: null })) };
                }
            } else {
                return { posts: [] };
            }
        } else if (query.keyword) {
            const keywords = query.keyword.split(/\s+/);
            const _posts = this.postStore.filter(post => {
                return keywords.some(keyword => post.title.includes(keyword));
            });

            if (_posts.length) {
                const [err, author] = await _try(services.UserService.getUser({ id: query.author }));

                if (!err && author) {
                    return { posts: _posts.map(post => ({ ...post, author })) };
                } else {
                    return { posts: _posts.map(post => ({ ...post, author: null })) };
                }
            } else {
                return { posts: [] };
            }
        } else {
            return { posts: [] };
        }
    }
}
