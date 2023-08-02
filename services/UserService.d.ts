import { ServiceClient } from "@hyurl/grpc-async";
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

export default interface UserService {
    getUser(query: UserQuery): Promise<User>;
    getMyPosts(query: UserQuery): Promise<UserPostList>;
}
