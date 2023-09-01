echo "generating code according to the proto files..."
protoc --proto_path=proto --go_out=./services --go-grpc_out=./services proto/*.proto
protoc --proto_path=proto --go_out=./services --go-grpc_out=./services proto/github/ayonli/services/*.proto
