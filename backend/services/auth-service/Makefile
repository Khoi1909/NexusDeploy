.PHONY: proto build run clean

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/auth.proto

build:
	go build -o bin/auth-service main.go

run:
	go run main.go

clean:
	rm -rf bin/
	rm -f proto/*.pb.go

deps:
	go mod tidy
	go mod download

docker-build:
	docker build -t nexus-auth-service .

docker-run:
	docker run --rm -p 8080:8080 -p 50051:50051 nexus-auth-service

