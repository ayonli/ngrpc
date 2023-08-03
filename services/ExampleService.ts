import { ServiceClient } from "@hyurl/grpc-async";

declare global {
    namespace services {
        const ExampleService: ServiceClient<ExampleService>;
    }
}

export type HelloRequest = {
    name: string;
};

export type HelloReply = {
    message: string;
};

export default class ExampleService {
    async sayHello(req: HelloRequest): Promise<HelloReply> {
        return await Promise.resolve({ message: "Hello, " + req.name });
    }
}
