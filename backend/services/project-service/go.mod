module github.com/nexusdeploy/backend/services/project-service

go 1.24.0

require (
	github.com/google/uuid v1.6.0
	github.com/nexusdeploy/backend/pkg/config v0.0.0
	github.com/nexusdeploy/backend/pkg/crypto v0.0.0
	github.com/nexusdeploy/backend/pkg/logger v0.0.0
	github.com/nexusdeploy/backend/services/project-service/proto v0.0.0
	github.com/rs/zerolog v1.34.0
	google.golang.org/grpc v1.67.0
	google.golang.org/protobuf v1.36.10
	gorm.io/driver/postgres v1.6.0
	gorm.io/gorm v1.31.1
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.6.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250804133106-a7a43d27e69b // indirect
)

replace github.com/nexusdeploy/backend/services/project-service/proto => ./proto

replace github.com/nexusdeploy/backend/pkg/config => ../../pkg/config

replace github.com/nexusdeploy/backend/pkg/crypto => ../../pkg/crypto

replace github.com/nexusdeploy/backend/pkg/logger => ../../pkg/logger
