echo "packing for macOS..."
GOOS=darwin GOARCH=amd64 go build -o prebuild/mac/amd64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
zip -j prebuild/ngrpc-mac-amd64.zip prebuild/mac/amd64/ngrpc
GOOS=darwin GOARCH=arm64 go build -o prebuild/mac/arm64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
zip -j prebuild/ngrpc-mac-arm64.zip prebuild/mac/arm64/ngrpc

echo "packing for Linux..."
GOOS=linux GOARCH=amd64 go build -o prebuild/linux/amd64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
zip -j prebuild/ngrpc-linux-amd64.zip prebuild/linux/amd64/ngrpc
GOOS=linux GOARCH=arm64 go build -o prebuild/linux/arm64/ngrpc github.com/ayonli/ngrpc/cli/ngrpc
zip -j prebuild/ngrpc-linux-arm64.zip prebuild/linux/arm64/ngrpc

echo "packing for Windows..."
GOOS=windows GOARCH=amd64 go build -o prebuild/windows/amd64/ngrpc.exe github.com/ayonli/ngrpc/cli/ngrpc
zip -j prebuild/ngrpc-windows-amd64.zip prebuild/windows/amd64/ngrpc.exe
GOOS=windows GOARCH=arm64 go build -o prebuild/windows/arm64/ngrpc.exe github.com/ayonli/ngrpc/cli/ngrpc
zip -j prebuild/ngrpc-windows-arm64.zip prebuild/windows/arm64/ngrpc.exe
