package sandbox

import (
	"context"
	"dev.rubentxu.devops-platform/core/domain"
	"fmt"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerContainerSandboxService struct {
	cli *client.Client
}

func NewDockerContainerSandboxService() (*DockerContainerSandboxService, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &DockerContainerSandboxService{cli: cli}, nil
}

func parseMemory(memory string) (int64, error) {
	// Implement logic to convert "1GB" to bytes (e.g., 1024 * 1024 * 1024)
	// For simplicity, let's assume memory is always in GB
	memoryInt, err := strconv.ParseInt(memory, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory: %v", err)
	}
	return memoryInt * 1024 * 1024 * 1024, nil // Convert GB to bytes
}

func parseDockerCPU(cpu string) (int64, error) {
	cpuInt, err := strconv.ParseInt(cpu, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU: %v", err)
	}
	return cpuInt, nil
}

func (d *DockerContainerSandboxService) CreateContainerSandbox(ctx context.Context, config domain.SandboxConfig) (string, error) {
	portBindings := make(nat.PortMap)
	exposedPorts := make(nat.PortSet)
	for containerPort, hostPort := range config.Network.PortBindings {
		port := nat.Port(containerPort + "/tcp")
		portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: hostPort}}
		exposedPorts[port] = struct{}{}
	}

	// Parse memory and CPU
	memory, err := parseMemory(config.Resources.Memory)
	if err != nil {
		return "", err
	}
	cpu, err := parseDockerCPU(config.Resources.CPU)
	if err != nil {
		return "", err
	}

	resp, err := d.cli.ContainerCreate(ctx, &container.Config{
		Image:        config.Image,
		Env:          mapToEnvVars(config.EnvVars),
		WorkingDir:   config.WorkingDir,
		ExposedPorts: exposedPorts,
	}, &container.HostConfig{
		PortBindings: portBindings,
		Resources: container.Resources{
			Memory:   memory,
			CPUCount: cpu,
		},
	}, nil, nil, "")
	if err != nil {
		return "", err
	}

	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", err
	}

	return "localhost:50051", nil
}

func mapToEnvVars(envVars map[string]string) []string {
	var env []string
	for key, value := range envVars {
		env = append(env, key+"="+value)
	}
	return env
}

func (d *DockerContainerSandboxService) DeleteContainerSandbox(ctx context.Context, sandboxID string) error {
	return d.cli.ContainerRemove(ctx, sandboxID, container.RemoveOptions{Force: true})
}

func (d *DockerContainerSandboxService) ListContainerSandboxes(ctx context.Context) ([]string, error) {
	containers, err := d.cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	var sandboxAddresses []string
	for _, container := range containers {
		sandboxAddresses = append(sandboxAddresses, fmt.Sprintf("localhost:%s", container.Ports[0].PublicPort))
	}
	return sandboxAddresses, nil
}
