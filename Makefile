build:
	go build -o bin/main cmd/api-gateway/main.go

run:
	bin/main

clean:
	rm -f bin/main

run-dev:
	go run cmd/api-gateway/main.go

docker-build:
	docker build -t stox-gateway .

docker-run:
	docker run -p 8080:8080 stox-gateway

docker-compose-up:
	docker-compose up -d

docker-compose-down:
	docker-compose down

.PHONY: build run clean run-dev docker-build docker-run docker-compose-up docker-compose-down

