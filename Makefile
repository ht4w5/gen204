BINARY := build/gen204

.PHONY: build test clean

build:
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/gen204

test:
	go test ./...

clean:
	rm -rf build
