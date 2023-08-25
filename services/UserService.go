package services

import (
	"context"
	"fmt"
	"slices"

	"github.com/ayonli/gorpc"
	"github.com/hyurl/grpc-boot/services/github/ayonli"
	"google.golang.org/grpc"
)

type UserService struct {
	ayonli.UnimplementedUserServiceServer
	userStore []*ayonli.User
	PostSrv   *PostService // set as exported field for dependency injection
}

func (self *UserService) Serve(s grpc.ServiceRegistrar) {
	ayonli.RegisterUserServiceServer(s, self)

	self.userStore = []*ayonli.User{
		{
			Id:     "ayon.li",
			Name:   "A-yon Lee",
			Gender: ayonli.Gender_MALE,
			Age:    28,
			Email:  "the@ayon.li",
		},
		{
			Id:     "john.doe",
			Name:   "John Doe",
			Gender: ayonli.Gender_UNKNOWN,
			Age:    -1,
			Email:  "john.doe@example.com",
		},
	}
}

func (self *UserService) Stop() {
	self.userStore = nil
	self.PostSrv = nil
}

func (self *UserService) Connect(cc grpc.ClientConnInterface) ayonli.UserServiceClient {
	return ayonli.NewUserServiceClient(cc)
}

func (self *UserService) GetClient(route string) ayonli.UserServiceClient {
	return gorpc.GetServiceClient(self, route)
}

func (self *UserService) GetUser(ctx context.Context, query *ayonli.UserQuery) (*ayonli.User, error) {
	if *query.Id != "" {
		idx := slices.IndexFunc[[]*ayonli.User](self.userStore, func(u *ayonli.User) bool {
			return u.Id == *query.Id
		})

		if idx != -1 {
			return self.userStore[idx], nil
		} else {
			return &ayonli.User{}, fmt.Errorf("User '%s' not found", *query.Id)
		}
	} else if *query.Email != "" {
		idx := slices.IndexFunc[[]*ayonli.User](self.userStore, func(u *ayonli.User) bool {
			return u.Email == *query.Email
		})

		if idx != -1 {
			return self.userStore[idx], nil
		} else {
			return &ayonli.User{}, fmt.Errorf("User of '%s' not found", *query.Email)
		}
	} else {
		return &ayonli.User{}, fmt.Errorf("One of the 'id' and 'email' must be provided")
	}
}

func (self *UserService) GetMyPosts(ctx context.Context, query *ayonli.UserQuery) (*ayonli.PostQueryResult, error) {
	user, err := self.GetUser(ctx, query)

	if err != nil {
		return &ayonli.PostQueryResult{}, err
	}

	ins := self.PostSrv.GetClient(user.Id)
	result, err := ins.SearchPosts(ctx, &ayonli.PostsQuery{Author: &user.Id})

	if err != nil {
		return &ayonli.PostQueryResult{}, err
	}

	return (*ayonli.PostQueryResult)(result), nil
}
