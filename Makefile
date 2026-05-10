.PHONY: build install clean test lint release sync-skill

BINARY   = btrack
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  = -ldflags "-X github.com/tolgazorlu/btrack/cmd.Version=$(VERSION) -s -w"
GOFLAGS  =

sync-skill:
	@rm -rf .claude/skills/btrack cmd/skill_data/btrack
	@mkdir -p .claude/skills cmd/skill_data
	@cp -R skills/btrack .claude/skills/btrack
	@cp -R skills/btrack cmd/skill_data/btrack
	@echo "synced skills/btrack/ -> .claude/skills/btrack/ + cmd/skill_data/btrack/"

build: sync-skill
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

release: sync-skill
	mkdir -p dist
	GOOS=linux   GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64  go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64  go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe .
	cd dist && sha256sum * > checksums.txt

proto:
	protoc --go_out=. --go-grpc_out=. proto/btrack.proto

run:
	go run . $(ARGS)
