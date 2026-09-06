.PHONY: build install test vet fmt

build:
	go build -o bin/chatgpt ./cmd/chatgpt

install:
	go install ./cmd/chatgpt

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal
