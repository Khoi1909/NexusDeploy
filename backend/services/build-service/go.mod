module github.com/nexusdeploy/backend/services/build-service

go 1.24.0

require (
	github.com/google/uuid v1.6.0
	github.com/hibiken/asynq v0.24.1
	github.com/nexusdeploy/backend/pkg/config v0.0.0
	github.com/nexusdeploy/backend/pkg/logger v0.0.0
	github.com/nexusdeploy/backend/services/build-service/proto v0.0.0
	github.com/rs/zerolog v1.33.0
	google.golang.org/grpc v1.72.1
	google.golang.org/protobuf v1.36.6
	gorm.io/driver/postgres v1.5.11
	gorm.io/gorm v1.25.12
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v5 v5.5.5 // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/redis/go-redis/v9 v9.0.3 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
)

replace (
	github.com/nexusdeploy/backend/pkg/config => ../../pkg/config
	github.com/nexusdeploy/backend/pkg/logger => ../../pkg/logger
	github.com/nexusdeploy/backend/services/build-service/proto => ./proto
)
