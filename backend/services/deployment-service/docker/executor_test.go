package docker

import (
	"testing"

	deploymentpb "github.com/nexusdeploy/backend/services/deployment-service/proto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestBuildEnvVars(t *testing.T) {
	log := zerolog.Nop()
	e := &Executor{
		log: log,
	}

	spec := &deploymentpb.DeploymentSpec{
		ProjectId: "test-project",
		Port:      3000,
		EnvVars: map[string]string{
			"NODE_ENV":    "production",
			"DATABASE_URL": "postgres://localhost/test",
		},
		Secrets: map[string]string{
			"API_KEY": "secret-api-key",
		},
	}

	envVars := e.buildEnvVars(spec)

	assert.Contains(t, envVars, "NODE_ENV=production")
	assert.Contains(t, envVars, "DATABASE_URL=postgres://localhost/test")
	assert.Contains(t, envVars, "API_KEY=secret-api-key")
	assert.Contains(t, envVars, "PORT=3000")
}

func TestBuildResourceLimits(t *testing.T) {
	log := zerolog.Nop()
	e := &Executor{
		log: log,
	}

	// Test with custom limits
	limits := &deploymentpb.ResourceLimits{
		MemoryMb: 1024,
		CpuCores: 2,
	}

	resources := e.buildResourceLimits(limits)

	assert.Equal(t, int64(1024*1024*1024), resources.Memory) // 1024 MB
	assert.Equal(t, int64(2000000000), resources.NanoCPUs)   // 2 CPU cores

	// Test with nil limits (defaults)
	resources = e.buildResourceLimits(nil)

	assert.Equal(t, int64(512*1024*1024), resources.Memory) // 512 MB default
	assert.Equal(t, int64(1000000000), resources.NanoCPUs)  // 1 CPU core default
}

func TestBuildTraefikLabels(t *testing.T) {
	log := zerolog.Nop()
	e := &Executor{
		log:                 log,
		traefikNetwork:      "nexus-network",
		traefikEntrypoint:   "web",
		traefikDomainSuffix: "localhost",
	}

	containerName := "nexus-app-test-abc"
	domain := "myapp.localhost"
	port := int32(8080)

	labels := e.buildTraefikLabels(containerName, domain, port)

	assert.Equal(t, "true", labels["traefik.enable"])
	assert.Equal(t, "Host(`myapp.localhost`)", labels["traefik.http.routers.nexus_app_test_abc.rule"])
	assert.Equal(t, "web", labels["traefik.http.routers.nexus_app_test_abc.entrypoints"])
	assert.Equal(t, "8080", labels["traefik.http.services.nexus_app_test_abc.loadbalancer.server.port"])
	assert.Equal(t, "nexus-network", labels["traefik.docker.network"])
	assert.Equal(t, "true", labels["io.nexusdeploy.managed"])
	assert.Equal(t, "myapp.localhost", labels["io.nexusdeploy.domain"])
}

func TestGetContainerName(t *testing.T) {
	log := zerolog.Nop()
	e := &Executor{
		log: log,
	}

	name := e.getContainerName("12345678-1234-1234-1234-123456789abc", "87654321-4321-4321-4321-abcdef123456")

	// Should truncate to 8 chars each
	assert.Equal(t, "nexus-app-12345678-87654321", name)
}

func TestGetDomain(t *testing.T) {
	log := zerolog.Nop()
	e := &Executor{
		log:                 log,
		traefikDomainSuffix: "example.com",
	}

	// With custom domain
	spec := &deploymentpb.DeploymentSpec{
		ProjectId: "test-project-id",
		Domain:    "custom.example.com",
	}
	domain := e.getDomain(spec)
	assert.Equal(t, "custom.example.com", domain)

	// Without custom domain (auto-generate)
	spec = &deploymentpb.DeploymentSpec{
		ProjectId: "test-project-id",
	}
	domain = e.getDomain(spec)
	assert.Equal(t, "test-pro.example.com", domain)
}

