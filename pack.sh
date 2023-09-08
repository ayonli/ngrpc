echo "packing for macOS..."
GOOS=darwin GOARCH=amd64 go build -o prebuild/mac/amd64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
tar -czvf prebuild/ngrpc-mac-amd64.tgz -C prebuild/mac/amd64  ngrpc
GOOS=darwin GOARCH=arm64 go build -o prebuild/mac/arm64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
tar -czvf prebuild/ngrpc-mac-arm64.tgz -C prebuild/mac/arm64  ngrpc

echo "packing for Linux..."
GOOS=linux GOARCH=amd64 go build -o prebuild/linux/amd64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
tar -czvf prebuild/ngrpc-linux-amd64.tgz -C prebuild/linux/amd64  ngrpc
GOOS=linux GOARCH=arm64 go build -o prebuild/linux/arm64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
tar -czvf prebuild/ngrpc-linux-arm64.tgz -C prebuild/linux/arm64  ngrpc

echo "packing for Windows..."
GOOS=windows GOARCH=amd64 go build -o prebuild/windows/amd64/ngrpc.exe github.com/ayonli/ngrpc/cli/ngrpc
tar -czvf prebuild/ngrpc-windows-amd64.tgz -C prebuild/windows/amd64  ngrpc.exe
GOOS=windows GOARCH=arm64 go build -o prebuild/windows/arm64/ngrpc.exe github.com/ayonli/ngrpc/cli/ngrpc
tar -czvf prebuild/ngrpc-windows-arm64.tgz -C prebuild/windows/arm64  ngrpc.exe
