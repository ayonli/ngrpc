package services

import (
	"context"
	"fmt"

	"golang.org/x/exp/slices"
)

type UserService struct {
	UnimplementedUserServiceServer
	userStore []*User
}

func (self *UserService) GetUser(ctx context.Context, query *UserQuery) (*User, error) {
	if *query.Id != "" {
		idx := slices.IndexFunc[[]*User](self.userStore, func(u *User) bool {
			return u.Id == *query.Id
		})

		if idx != -1 {
			return self.userStore[idx], nil
		} else {
			return &User{}, fmt.Errorf("User '%s' not found", *query.Id)
		}
	} else if *query.Email != "" {
		idx := slices.IndexFunc[[]*User](self.userStore, func(u *User) bool {
			return u.Email == *query.Email
		})

		if idx != -1 {
			return self.userStore[idx], nil
		} else {
			return &User{}, fmt.Errorf("User of '%s' not found", *query.Email)
		}
	} else {
		return &User{}, fmt.Errorf("One of the 'id' and 'email' must be provided")
	}
}

func (self *UserService) GetMyPosts(ctx context.Context, query *UserQuery) (*PostQueryResult, error) {
	user, err := self.GetUser(ctx, query)

	if err != nil {
		return &PostQueryResult{}, err
	}

	result, err := GetPostServiceInstance().SearchPosts(ctx, &PostsQuery{Author: &user.Id})

	if err != nil {
		return &PostQueryResult{}, err
	}

	return (*PostQueryResult)(result), nil
}

func NewUserService() *UserService {
	service := &UserService{}

	service.userStore = append(service.userStore, &User{
		Id:     "ayon.li",
		Name:   "A-yon Lee",
		Gender: Gender_MALE,
		Age:    28,
		Email:  "the@ayon.li",
	}, &User{
		Id:     "john.doe",
		Name:   "John Doe",
		Gender: Gender_UNKNOWN,
		Age:    -1,
		Email:  "john.doe@example.com",
	})

	return service
}
