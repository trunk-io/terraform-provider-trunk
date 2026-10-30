default: build

build:
	go build ./...

fmt:
	gofmt -w .

test:
	go test ./...

testacc:
	TF_ACC=1 go test ./... -timeout 10m

lint:
	golangci-lint run ./...

generate:
	go generate ./...

.PHONY: default build fmt test testacc lint generate
