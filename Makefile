build:
	go build -o bin/main cmd/api-gateway/main.go

run:
	bin/main

clean:
	rm -f bin/main

.PHONY: build run clean

run-dev:
	go run cmd/api-gateway/main.go