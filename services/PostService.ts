import _try from "dotry";
import { ServiceClient, LifecycleSupportInterface, service } from "@ayonli/ngrpc";
import { Post, User } from "./struct";

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

@service("github.ayonli.ngrpc.services.PostService")
export default class PostService implements LifecycleSupportInterface {
    private userSrv = services.UserService;
    private postStore: (Omit<Post, "author"> & { author: string; })[] | null = null;

    async init(): Promise<void> {
        this.postStore = [
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
                content: "The article contents ...",
                author: "ayon.li",
            }
        ];
    }

    async destroy(): Promise<void> {
        this.postStore = null;
    }

    async getPost(query: PostQuery): Promise<Post> {
        const post = this.postStore?.find(item => item.id === query.id);

        if (post) {
            const author = await this.userSrv.getUser({ id: post.author });
            return { ...post, author };
        } else {
            throw new Error(`Post ${query.id} not found`);
        }
    }

    async searchPosts(query: PostsQuery): Promise<PostSearchResult> {
        if (query.author) {
            const _posts = this.postStore?.filter(item => item.author === query.author);

            if (_posts?.length) {
                const { users } = await this.userSrv.getUsers({});
                return {
                    posts: _posts.map(post => {
                        const author = users.find(user => user.id === post.author) as User;
                        return { ...post, author };
                    }),
                };
            } else {
                return { posts: [] };
            }
        } else if (query.keyword) {
            const keywords = query.keyword.split(/\s+/);
            const _posts = this.postStore?.filter(post => {
                return keywords.some(keyword => post.title.includes(keyword));
            });

            if (_posts?.length) {
                const { users } = await this.userSrv.getUsers({});
                return {
                    posts: _posts.map(post => {
                        const author = users.find(user => user.id === post.author) as User;
                        return { ...post, author };
                    }),
                };
            } else {
                return { posts: [] };
            }
        } else {
            return { posts: [] };
        }
    }
}
