.PHONY: build install clean test fmt vet

# Build the binary
build:
	go build -o dvm ./cmd/dvm

# Install to /usr/local/bin
install: build
	sudo mv dvm /usr/local/bin/

# Clean build artifacts
clean:
	rm -f dvm
	go clean

# Run tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run all checks
check: fmt vet test

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o dist/dvm-linux-amd64 ./cmd/dvm
	GOOS=linux GOARCH=arm64 go build -o dist/dvm-linux-arm64 ./cmd/dvm
	GOOS=darwin GOARCH=amd64 go build -o dist/dvm-darwin-amd64 ./cmd/dvm
	GOOS=darwin GOARCH=arm64 go build -o dist/dvm-darwin-arm64 ./cmd/dvm
	GOOS=windows GOARCH=amd64 go build -o dist/dvm-windows-amd64.exe ./cmd/dvm

# Download dependencies
deps:
	go mod download
	go mod tidy
