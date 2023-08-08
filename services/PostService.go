package services

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"os"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type PostService struct{}

var singleton PostServiceClient
var lock = &sync.Mutex{}

func GetPostServiceInstance() PostServiceClient {
	if singleton == nil {
		lock.Lock()
		defer lock.Unlock()

		if singleton == nil {
			ca, err := os.ReadFile("certs/ca.pem")

			if err != nil {
				log.Fatalf("Error loading CA: %v", err)
			}

			certPool := x509.NewCertPool()

			if ok := certPool.AppendCertsFromPEM(ca); !ok {
				log.Fatalf("Unable to create cert pool: %v", err)
			}

			cert, err := tls.LoadX509KeyPair("certs/cert.pem", "certs/cert.key")

			if err != nil {
				log.Fatalf("Error loading Cert: %v", err)
			}

			config := &tls.Config{
				Certificates: []tls.Certificate{cert},
				RootCAs:      certPool,
			}

			conn, err := grpc.Dial("localhost:4002",
				grpc.WithTransportCredentials(credentials.NewTLS(config)))

			if err != nil {
				log.Fatalf("Cannot not connect: %v", err)
			}

			singleton = NewPostServiceClient(conn)
		}
	}

	return singleton
}
