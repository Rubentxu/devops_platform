package sandbox

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"dev.rubentxu.devops-platform/core/domain"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type DockerContainerSandboxService struct {
	cli           *client.Client
	usedAddresses map[string]bool
	mu            sync.Mutex
	host          string
	minPort       int
	maxPort       int
}

func NewDockerContainerSandboxService() (*DockerContainerSandboxService, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}

	// Obtener el host del Docker daemon
	host := cli.DaemonHost()

	return &DockerContainerSandboxService{
		cli:           cli,
		usedAddresses: make(map[string]bool),
		host:          host,
		minPort:       30000,
		maxPort:       40000,
	}, nil
}

func (d *DockerContainerSandboxService) getAvailableAddress() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	port := rand.Intn(d.maxPort-d.minPort) + d.minPort
	host := d.host

	if strings.HasPrefix(host, "unix://") || host == "" {
		// Asumimos local
		return fmt.Sprintf("http://localhost:%d", port), nil
	}

	// Remoto
	host = strings.TrimPrefix(host, "tcp://")
	host = strings.Split(host, ":")[0] // Extraer solo el hostname/IP
	return fmt.Sprintf("http://%s:%d", host, port), nil
}

func (d *DockerContainerSandboxService) releaseAddress(address string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.usedAddresses, address)
}

func isAddressAvailable(address string) bool {
	conn, err := net.DialTimeout("tcp", address, 1*time.Second)
	if err != nil {
		return true
	}
	conn.Close()
	return false
}

func parseMemory(memory string) (int64, error) {
	if memory == "" {
		return 0, fmt.Errorf("memory configuration is required")
	}

	// Convertir memoria a bytes
	switch {
	case strings.HasSuffix(memory, "GB"):
		gb, err := strconv.ParseInt(strings.TrimSuffix(memory, "GB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory format: %w", err)
		}
		return gb * 1024 * 1024 * 1024, nil
	case strings.HasSuffix(memory, "MB"):
		mb, err := strconv.ParseInt(strings.TrimSuffix(memory, "MB"), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid memory format: %w", err)
		}
		return mb * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unsupported memory unit, use GB or MB")
	}
}

func parseDockerCPU(cpu string) (int64, error) {
	cpuInt, err := strconv.ParseInt(cpu, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse CPU: %v", err)
	}
	return cpuInt, nil
}

func (d *DockerContainerSandboxService) CreateContainerSandbox(ctx context.Context, config *domain.SandboxConfig) (string, error) {
	// Obtener address disponible
	address, err := d.getAvailableAddress()
	if err != nil {
		return "", fmt.Errorf("failed to get available address: %w", err)
	}

	// Configurar red si no está configurada
	config.Network = domain.NetworkConfig{
		ExposedPorts: make(map[string]struct{}),
		PortBindings: make(map[string]string),
	}

	// Extraer el puerto del address
	address = strings.TrimPrefix(address, "http://")
	address = strings.TrimPrefix(address, "https://")
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("invalid address format: %w", err)
	}

	// Configurar puertos
	containerPort := fmt.Sprintf("%v/tcp", port) // Puerto interno del contenedor
	config.Network.ExposedPorts[containerPort] = struct{}{}
	config.Network.PortBindings[containerPort] = port

	// Validar configuración
	if config.Image == "" {
		return "", fmt.Errorf("image is required")
	}

	portBindings := make(nat.PortMap)
	exposedPorts := make(nat.PortSet)
	for containerPort, hostPort := range config.Network.PortBindings {
		port := nat.Port(containerPort)
		portBindings[port] = []nat.PortBinding{{HostIP: "0.0.0.0", HostPort: fmt.Sprintf("%v", hostPort)}}
		exposedPorts[port] = struct{}{}
	}

	// Configuración de recursos
	var memory, cpu int64
	if config.Resources.Memory != "" {
		memory, err = parseMemory(config.Resources.Memory)
		if err != nil {
			return "", fmt.Errorf("invalid memory configuration: %w", err)
		}
	}

	if config.Resources.CPU != "" {
		cpu, err = parseDockerCPU(config.Resources.CPU)
		if err != nil {
			return "", fmt.Errorf("invalid CPU configuration: %w", err)
		}
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Resources: container.Resources{
			Memory:   memory,
			CPUCount: cpu,
		},
	}

	configContainer := &container.Config{
		Hostname: config.Name,
		Image:    config.Image,
		//Env:      mapToEnvVars(config.EnvVars),
		//WorkingDir:   config.WorkingDir,
		//ExposedPorts: exposedPorts,
	}

	fmt.Printf("Config: %+v\n", configContainer)
	fmt.Printf("HostConfig: %+v\n", hostConfig)

	resp, err := d.cli.ContainerCreate(ctx, configContainer, nil, nil, nil, config.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	if err := d.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = d.cli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	// Generar URL basada en el host y el puerto mapeado
	var serviceURL string

	return serviceURL, nil
}

func mapToEnvVars(envVars map[string]string) []string {
	env := make([]string, 0, len(envVars))
	for key, value := range envVars {
		env = append(env, key+"="+value)
	}
	return env
}

func (d *DockerContainerSandboxService) DeleteContainerSandbox(ctx context.Context, sandboxID string) error {
	// Obtener información del contenedor para liberar el address
	info, err := d.cli.ContainerInspect(ctx, sandboxID)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	// Liberar address
	for _, binding := range info.HostConfig.PortBindings {
		for _, portBinding := range binding {
			address := fmt.Sprintf("%s:%s", d.host, portBinding.HostPort)
			d.releaseAddress(address)
		}
	}

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
