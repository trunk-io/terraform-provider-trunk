default: build

build:
	go build ./...

fmt:
	gofmt -w .

test:
	go test ./internal/client/ -v

testacc:
	TF_ACC=1 go test ./internal/provider/ -v -timeout 10m

lint:
	golangci-lint run ./...

generate:
	go generate ./...

.PHONY: default build fmt test testacc lint generate
