package services

import (
	"context"
	"fmt"
	"slices"

	"github.com/ayonli/goext"
	"github.com/ayonli/ngrpc"
	"github.com/ayonli/ngrpc/services/github/ayonli/services_proto"
	"google.golang.org/grpc"
)

type UserService struct {
	services_proto.UnimplementedUserServiceServer
	userStore []*services_proto.User
	PostSrv   *PostService // set as exported field for dependency injection
}

func (self *UserService) Serve(s grpc.ServiceRegistrar) {
	services_proto.RegisterUserServiceServer(s, self)

	self.userStore = []*services_proto.User{
		{
			Id:     "ayon.li",
			Name:   "A-yon Lee",
			Gender: services_proto.Gender_MALE,
			Age:    28,
			Email:  "the@ayon.li",
		},
		{
			Id:     "john.doe",
			Name:   "John Doe",
			Gender: services_proto.Gender_UNKNOWN,
			Age:    -1,
			Email:  "john.doe@example.com",
		},
	}
}

func (self *UserService) Stop() {
	self.userStore = nil
	self.PostSrv = nil
}

func (self *UserService) Connect(cc grpc.ClientConnInterface) services_proto.UserServiceClient {
	return services_proto.NewUserServiceClient(cc)
}

func (self *UserService) GetClient(route string) (services_proto.UserServiceClient, error) {
	return ngrpc.GetServiceClient(self, route)
}

func (self *UserService) GetUser(ctx context.Context, query *services_proto.UserQuery) (*services_proto.User, error) {
	if *query.Id != "" {
		idx := slices.IndexFunc[[]*services_proto.User](self.userStore, func(u *services_proto.User) bool {
			return u.Id == *query.Id
		})

		if idx != -1 {
			return self.userStore[idx], nil
		} else {
			return nil, fmt.Errorf("user '%s' not found", *query.Id)
		}
	} else if *query.Email != "" {
		idx := slices.IndexFunc[[]*services_proto.User](self.userStore, func(u *services_proto.User) bool {
			return u.Email == *query.Email
		})

		if idx != -1 {
			return self.userStore[idx], nil
		} else {
			return nil, fmt.Errorf("user of '%s' not found", *query.Email)
		}
	} else {
		return nil, fmt.Errorf("one of the 'id' and 'email' must be provided")
	}
}

func (self *UserService) GetMyPosts(
	ctx context.Context,
	query *services_proto.UserQuery,
) (*services_proto.PostQueryResult, error) {
	return goext.Try(func() *services_proto.PostQueryResult {
		user := goext.Ok(self.GetUser(ctx, query))
		ins := goext.Ok(self.PostSrv.GetClient(user.Id))
		result := goext.Ok(ins.SearchPosts(ctx, &services_proto.PostsQuery{Author: &user.Id}))

		return (*services_proto.PostQueryResult)(result)
	})
}

func init() {
	ngrpc.Use(&UserService{})
}
