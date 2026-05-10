.PHONY: build install clean test lint release

BINARY   = btrack
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  = -ldflags "-X github.com/tolgazorlu/btrack/cmd.Version=$(VERSION) -s -w"
GOFLAGS  =

build:
	go build $(LDFLAGS) -o $(BINARY) .

install: build
	install -m 755 $(BINARY) /usr/local/bin/$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf dist/

test:
	go test ./...

lint:
	golangci-lint run ./...

# Cross-platform release builds
release:
	mkdir -p dist
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe .
	cd dist && sha256sum * > checksums.txt

# Generate gRPC stubs (requires protoc + protoc-gen-go + protoc-gen-go-grpc)
proto:
	protoc --go_out=. --go-grpc_out=. proto/btrack.proto

run:
	go run . $(ARGS)
