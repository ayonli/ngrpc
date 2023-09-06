package services

import (
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services/github/ayonli/ngrpc/services_proto"
	"google.golang.org/grpc"
)

type PostService struct{}

func (self *PostService) Connect(cc grpc.ClientConnInterface) services_proto.PostServiceClient {
	return services_proto.NewPostServiceClient(cc)
}

func (self *PostService) GetClient(route string) (services_proto.PostServiceClient, error) {
	return ngrpc.GetServiceClient(self, route)
}

func init() {
	ngrpc.Use(&PostService{})
}
