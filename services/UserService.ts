import { ServiceClient } from "..";
import { service } from "../util";
import { Post, User } from "./struct";

declare global {
    namespace services {
        const UserService: ServiceClient<UserService>;
    }
}

export type UserQuery = {
    id?: string;
    email?: string;
};

export type UserPostList = {
    author: User;
    posts: Post[];
};

@service("github.ayonli.services.UserService")
export default abstract class UserService {
    abstract getUser(query: UserQuery): Promise<User>;
    abstract getMyPosts(query: UserQuery): Promise<UserPostList>;
}
