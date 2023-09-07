echo "building for macOS..."
GOOS=darwin GOARCH=amd64 go build -o prebuild/mac/amd64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
GOOS=darwin GOARCH=arm64 go build -o prebuild/mac/arm64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc

echo "biulding for Linux..."
GOOS=linux GOARCH=amd64 go build -o prebuild/linux/amd64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
GOOS=linux GOARCH=arm64 go build -o prebuild/linux/arm64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc

echo "biulding for Windows..."
GOOS=windows GOARCH=amd64 go build -o prebuild/windows/amd64/ngrpc.exe github.com/ayonli/ngrpc/cli/ngrpc
GOOS=windows GOARCH=arm64 go build -o prebuild/windows/arm64/ngrpc.exe github.com/ayonli/ngrpc/cli/ngrpc
