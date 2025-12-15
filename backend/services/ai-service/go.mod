module github.com/nexusdeploy/backend/services/ai-service

go 1.24.0

require google.golang.org/grpc v1.72.1

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/nexusdeploy/backend/pkg/config v0.0.0 // indirect
	github.com/nexusdeploy/backend/pkg/grpc v0.0.0 // indirect
	github.com/nexusdeploy/backend/pkg/logger v0.0.0 // indirect
	github.com/nexusdeploy/backend/services/ai-service/proto v0.0.0 // indirect
	github.com/nexusdeploy/backend/services/build-service/proto v0.0.0 // indirect
	github.com/redis/go-redis/v9 v9.6.1 // indirect
	github.com/rs/zerolog v1.33.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

replace (
	github.com/nexusdeploy/backend/pkg/config => ../../pkg/config
	github.com/nexusdeploy/backend/pkg/grpc => ../../pkg/grpc
	github.com/nexusdeploy/backend/pkg/logger => ../../pkg/logger
	github.com/nexusdeploy/backend/services/ai-service/proto => ./proto
	github.com/nexusdeploy/backend/services/build-service/proto => ../build-service/proto
)
