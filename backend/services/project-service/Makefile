.PHONY: proto build run clean

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/project.proto

build:
	go build -o bin/project-service main.go

run:
	go run main.go

clean:
	rm -rf bin/
	rm -f proto/*.pb.go

deps:
	go mod tidy
	go mod download

docker-build:
	docker build -t nexus-project-service -f ../auth-service/Dockerfile .

docker-run:
	docker run --rm -p 8081:8081 -p 50052:50052 nexus-project-service

