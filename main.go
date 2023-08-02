package main

import (
	"log"
	"net"

	"github.com/hyurl/grpc-boot/services"
	"google.golang.org/grpc"
)

func main() {
	addr := "localhost:4001"
	tcpSrv, err := net.Listen("tcp", addr)

	if err != nil {
		log.Fatalf("Failed to listen on port %s", addr)
	}

	grpcSrv := grpc.NewServer()
	services.RegisterUserServiceServer(grpcSrv, services.NewUserService())

	log.Printf("server listening at %v", tcpSrv.Addr())

	if err := grpcSrv.Serve(tcpSrv); err != nil {
		log.Fatalln("Failed to start the gRPC server")
	}
}
