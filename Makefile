SHELL=/bin/bash

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: lint
lint:
	golangci-lint run
	staticcheck ./...
