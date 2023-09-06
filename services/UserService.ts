import { ServiceClient, service } from "@ayonli/ngrpc";
import { Gender, User } from "./struct";
import { PostSearchResult } from "./PostService";

declare global {
    namespace services {
        const UserService: ServiceClient<UserService>;
    }
}

export type UserQuery = {
    id?: string;
    email?: string;
};

export type UsersQuery = {
    gender?: Gender;
    minAge?: number;
    maxAge?: number;
};

export type UserQueryResult = {
    users: User[];
};

@service("github.ayonli.ngrpc.services.UserService")
export default abstract class UserService {
    abstract getUser(query: UserQuery): Promise<User>;
    abstract getUsers(query: UsersQuery): Promise<UserQueryResult>;
    abstract getMyPosts(query: UserQuery): Promise<PostSearchResult>;
}
