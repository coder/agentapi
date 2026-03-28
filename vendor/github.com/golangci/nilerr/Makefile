.PHONY: clean test build

default: clean test build

clean:
	rm -rf dist/ cover.out

test: clean
	go test -v -cover ./...

build:
	go build -ldflags "-s -w" -trimpath ./cmd/nilerr/
