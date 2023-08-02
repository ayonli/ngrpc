package services

import (
	"log"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type PostService struct{}

var singleton PostServiceClient
var lock = &sync.Mutex{}

func GetPostServiceInstance() PostServiceClient {
	if singleton == nil {
		lock.Lock()
		defer lock.Unlock()

		if singleton == nil {
			conn, err := grpc.Dial("localhost:4002",
				grpc.WithTransportCredentials(insecure.NewCredentials()))

			if err != nil {
				log.Fatalf("Cannot not connect: %v", err)
			}

			singleton = NewPostServiceClient(conn)
		}
	}

	return singleton
}
