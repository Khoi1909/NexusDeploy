module github.com/nexusdeploy/tests

go 1.23

require (
	github.com/nexusdeploy/backend/services/auth-service/proto v0.0.0
	github.com/nexusdeploy/backend/services/project-service/proto v0.0.0
	google.golang.org/grpc v1.61.0
)

replace github.com/nexusdeploy/backend/services/auth-service/proto => ../backend/services/auth-service/proto
replace github.com/nexusdeploy/backend/services/project-service/proto => ../backend/services/project-service/proto
