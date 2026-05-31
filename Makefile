BINARY=nota
CMD=./cmd/nota
VERSION?=0.1.0
LDFLAGS=-ldflags "-X github.com/soriano/nota/internal/cli.Version=$(VERSION)"

.PHONY: build build-all clean install test

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 $(CMD)

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 $(CMD)

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 $(CMD)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 $(CMD)

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe $(CMD)

build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64

clean:
	rm -f $(BINARY) $(BINARY)-*

install: build
	cp $(BINARY) /usr/local/bin/$(BINARY)

test:
	go test ./...
