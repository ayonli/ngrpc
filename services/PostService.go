package services

import (
	"github.com/ayonli/gorpc"
	"github.com/hyurl/grpc-boot/services/github/ayonli"
	"google.golang.org/grpc"
)

type PostService struct{}

func (self *PostService) Connect(cc grpc.ClientConnInterface) ayonli.PostServiceClient {
	return ayonli.NewPostServiceClient(cc)
}

func (self *PostService) GetClient(route string) ayonli.PostServiceClient {
	return gorpc.GetServiceClient(self, route)
}
