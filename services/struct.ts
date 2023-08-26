export enum Gender {
    UNKNOWN = 0,
    MALE = 1,
    FEMALE = 2,
}

export type User = {
    id: string;
    name: string;
    gender: Gender;
    age: number;
    email: string;
};

export type Post = {
    id: number;
    title: string;
    description?: string;
    content: string;
    author: User | null;
};
