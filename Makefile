.PHONY: all build relay agent test clean

all: build

build: relay agent

relay:
	go build -o bin/relay ./cmd/relay

agent:
	go build -o bin/agent ./cmd/agent

test:
	go test ./...

clean:
	rm -rf bin/
