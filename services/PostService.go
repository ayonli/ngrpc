package services

import (
	"github.com/ayonli/gorpc"
	"github.com/hyurl/grpc-boot/services/github/ayonli/services_proto"
	"google.golang.org/grpc"
)

type PostService struct{}

func (self *PostService) Connect(cc grpc.ClientConnInterface) services_proto.PostServiceClient {
	return services_proto.NewPostServiceClient(cc)
}

func (self *PostService) GetClient(route string) services_proto.PostServiceClient {
	return gorpc.GetServiceClient(self, route)
}
