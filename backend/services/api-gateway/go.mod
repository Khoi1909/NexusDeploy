module github.com/nexusdeploy/backend/services/api-gateway

go 1.24.0

require (
	github.com/gorilla/websocket v1.5.3
	github.com/nexusdeploy/backend/pkg/config v0.0.0
	github.com/nexusdeploy/backend/pkg/grpc v0.0.0
	github.com/nexusdeploy/backend/pkg/logger v0.0.0
	github.com/nexusdeploy/backend/pkg/middleware v0.0.0
	github.com/nexusdeploy/backend/services/auth-service/proto v0.0.0
	github.com/nexusdeploy/backend/services/build-service/proto v0.0.0
	github.com/nexusdeploy/backend/services/deployment-service/proto v0.0.0
	github.com/nexusdeploy/backend/services/project-service/proto v0.0.0
	github.com/prometheus/client_golang v1.23.2
	github.com/rs/zerolog v1.33.0
	google.golang.org/grpc v1.77.0
	google.golang.org/protobuf v1.36.10
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

replace github.com/nexusdeploy/backend/pkg/config => ../../pkg/config

replace github.com/nexusdeploy/backend/pkg/grpc => ../../pkg/grpc

replace github.com/nexusdeploy/backend/pkg/logger => ../../pkg/logger

replace github.com/nexusdeploy/backend/pkg/middleware => ../../pkg/middleware

replace github.com/nexusdeploy/backend/services/auth-service/proto => ../auth-service/proto

replace github.com/nexusdeploy/backend/services/project-service/proto => ../project-service/proto

replace github.com/nexusdeploy/backend/services/build-service/proto => ../build-service/proto

replace github.com/nexusdeploy/backend/services/deployment-service/proto => ../deployment-service/proto
