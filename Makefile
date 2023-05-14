.PHONY: deps
deps:
	go mod download
	go mod tidy

.PHONY: bulid
build: deps
	go build -o bin/review main.go
