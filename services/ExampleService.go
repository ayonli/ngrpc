package services

import (
	"context"

	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services/proto"
	"google.golang.org/grpc"
)

type ExampleService struct {
	proto.UnimplementedExampleServiceServer
}

func (self *ExampleService) Serve(s grpc.ServiceRegistrar) {
	proto.RegisterExampleServiceServer(s, self)
}

func (self *ExampleService) Connect(cc grpc.ClientConnInterface) proto.ExampleServiceClient {
	return proto.NewExampleServiceClient(cc)
}

func (self *ExampleService) GetClient(route string) (proto.ExampleServiceClient, error) {
	return ngrpc.GetServiceClient(self, route)
}

func (self *ExampleService) SayHello(ctx context.Context, req *proto.HelloRequest) (*proto.HelloReply, error) {
	return &proto.HelloReply{Message: "Hello, " + req.Name}, nil
}

func init() {
	ngrpc.Use(&ExampleService{})
}
