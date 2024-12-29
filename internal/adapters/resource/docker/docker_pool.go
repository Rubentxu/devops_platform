package docker

import (
	"context"
	"dev.rubentxu.devops-platform/core/domain/resource"
	"dev.rubentxu.devops-platform/core/domain/task"
	"dev.rubentxu.devops-platform/core/ports"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/pkg/errors"
	"dev.rubentxu.devops-platform/internal/adapters/grpc/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// DockerHostPool implementa ResourcePool usando Docker
type DockerHostPool struct {
	id            string
	name          string
	endpoint      string
	client        *client.Client
	logger        ports.Logger
	resourceStats *resource.ResourceStats
	networkID     string
	managerID     string
	workerIDs     []string
	workerClient  *client.WorkerClient
}

// NewDockerHostPool crea una nueva instancia de DockerHostPool
func NewDockerHostPool(ctx context.Context, id string, name string, endpoint string, logger ports.Logger) (*DockerHostPool, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(endpoint),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create docker client")
	}

	pool := &DockerHostPool{
		id:        id,
		name:      name,
		endpoint:  endpoint,
		client:    cli,
		logger:    logger.With("component", "docker_pool", "id", id),
		workerIDs: make([]string, 0),
	}

	// Crear red
	if err := pool.createNetwork(ctx); err != nil {
		return nil, err
	}

	// Iniciar manager
	if err := pool.startManager(ctx); err != nil {
		return nil, err
	}

	// Iniciar workers
	if err := pool.startWorkers(ctx, 2); err != nil {
		return nil, err
	}

	// Inicializar cliente gRPC
	if err := pool.initWorkerClient(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}

func (p *DockerHostPool) initWorkerClient(ctx context.Context) error {
	// Obtener el endpoint del manager
	managerContainer, err := p.client.ContainerInspect(ctx, p.managerID)
	if err != nil {
		return errors.Wrap(err, "failed to inspect manager container")
	}

	managerIP := managerContainer.NetworkSettings.Networks[p.networkID].IPAddress
	managerEndpoint := fmt.Sprintf("%s:50051", managerIP)

	// Establecer conexión gRPC
	conn, err := grpc.Dial(managerEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return errors.Wrap(err, "failed to create gRPC connection")
	}

	p.workerClient = client.NewWorkerClient(conn)
	return nil
}

func (p *DockerHostPool) createNetwork(ctx context.Context) error {
	networkName := fmt.Sprintf("devops-platform-%s", p.id)

	// Verificar si la red ya existe
	networks, err := p.client.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list networks")
	}

	for _, n := range networks {
		if n.Name == networkName {
			p.networkID = n.ID
			return nil
		}
	}

	// Crear nueva red
	resp, err := p.client.NetworkCreate(ctx, networkName, types.NetworkCreate{
		Driver: "bridge",
		Labels: map[string]string{
			"dev.rubentxu.devops-platform": "true",
			"pool.id":                      p.id,
		},
	})
	if err != nil {
		return errors.Wrap(err, "failed to create network")
	}

	p.networkID = resp.ID
	return nil
}

func (p *DockerHostPool) startManager(ctx context.Context) error {
	managerName := fmt.Sprintf("manager-%s", p.id)

	config := &container.Config{
		Image: "devops-platform-manager:latest",
		ExposedPorts: nat.PortSet{
			"50051/tcp": struct{}{},
		},
		Env: []string{
			"MANAGER_PORT=50051",
		},
		Labels: map[string]string{
			"dev.rubentxu.devops-platform": "true",
			"pool.id":                      p.id,
			"role":                         "manager",
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"50051/tcp": []nat.PortBinding{{HostPort: "50051"}},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			p.networkID: {
				NetworkID: p.networkID,
			},
		},
	}

	resp, err := p.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, managerName)
	if err != nil {
		return errors.Wrap(err, "failed to create manager container")
	}

	if err := p.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return errors.Wrap(err, "failed to start manager container")
	}

	p.managerID = resp.ID
	p.logger.Info("manager container started", "container_id", resp.ID)

	// Esperar a que el manager esté listo
	time.Sleep(5 * time.Second)
	return nil
}

func (p *DockerHostPool) startWorkers(ctx context.Context, count int) error {
	// Obtener la IP del manager
	managerContainer, err := p.client.ContainerInspect(ctx, p.managerID)
	if err != nil {
		return errors.Wrap(err, "failed to inspect manager container")
	}

	managerIP := managerContainer.NetworkSettings.Networks[p.networkID].IPAddress

	for i := 0; i < count; i++ {
		workerName := fmt.Sprintf("worker-%s-%d", p.id, i)

		config := &container.Config{
			Image: "devops-platform-worker:latest",
			Env: []string{
				fmt.Sprintf("MANAGER_HOST=%s", managerIP),
				"MANAGER_PORT=50051",
			},
			Labels: map[string]string{
				"dev.rubentxu.devops-platform": "true",
				"pool.id":                      p.id,
				"role":                         "worker",
			},
		}

		hostConfig := &container.HostConfig{
			RestartPolicy: container.RestartPolicy{
				Name: "unless-stopped",
			},
		}

		networkingConfig := &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				p.networkID: {
					NetworkID: p.networkID,
				},
			},
		}

		resp, err := p.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, workerName)
		if err != nil {
			return errors.Wrap(err, "failed to create worker container")
		}

		if err := p.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
			return errors.Wrap(err, "failed to start worker container")
		}

		p.workerIDs = append(p.workerIDs, resp.ID)
		p.logger.Info("worker container started", "container_id", resp.ID)
	}

	return nil
}

// GetID implementa ResourcePool.GetID
func (p *DockerHostPool) GetID() string {
	return p.id
}

// GetStats implementa ResourcePool.GetStats
func (p *DockerHostPool) GetStats() resource.ResourceStats {
	ctx := context.Background()
	info, err := p.client.Info(ctx)
	if err != nil {
		p.logger.Error("failed to get docker info", "error", err)
		return resource.NewResourceStats()
	}

	// Obtener estadísticas del manager
	stats, err := p.client.ContainerStats(ctx, p.managerID, false)
	if err != nil {
		return resource.NewResourceStats()
	}
	defer stats.Body.Close()

	var containerStats types.StatsJSON
	if err := json.NewDecoder(stats.Body).Decode(&containerStats); err != nil {
		return resource.NewResourceStats()
	}

	cpuUsage := float64(containerStats.CPUStats.CPUUsage.TotalUsage)
	memoryUsage := float64(containerStats.MemoryStats.Usage)
	cpuCount := float64(info.NCPU)

	return resource.ResourceStats{
		CPUStats: resource.CPUStats{
			UsagePercentage: (cpuUsage / cpuCount) * 100,
			AvailableCores:  info.NCPU,
		},
		MemoryStats: resource.MemoryStats{
			TotalKb: uint64(info.MemTotal) / 1024,
			UsedKb:  uint64(memoryUsage) / 1024,
			FreeKb:  (uint64(info.MemTotal) - uint64(memoryUsage)) / 1024,
		},
		DiskStats: resource.DiskStats{
			TotalBytes: 0,
			UsedBytes:  0,
			FreeBytes:  0,
		},
	}
}

// GetHealthStatus implementa ResourcePool.GetHealthStatus
func (p *DockerHostPool) GetHealthStatus() resource.HealthStatus {
	ctx := context.Background()

	// Verificar estado del manager
	managerContainer, err := p.client.ContainerInspect(ctx, p.managerID)
	if err != nil || !managerContainer.State.Running {
		return resource.HealthStatus{
			IsHealthy: false,
			LastCheck: time.Now(),
			Message:   "Manager container is not running",
		}
	}

	// Verificar estado de los workers
	for i, workerID := range p.workerIDs {
		workerContainer, err := p.client.ContainerInspect(ctx, workerID)
		if err != nil || !workerContainer.State.Running {
			return resource.HealthStatus{
				IsHealthy: false,
				LastCheck: time.Now(),
				Message:   fmt.Sprintf("Worker %d is not running", i),
			}
		}
	}

	return resource.NewHealthStatus()
}

// ExecuteTask implementa ResourcePool.ExecuteTask
func (p *DockerHostPool) ExecuteTask(ctx context.Context, t *task.Task) error {
	p.logger.Info("executing task",
		"task_id", t.Metadata.ID,
		"name", t.Metadata.Name)

	// Preparar el comando y variables de entorno
	command := t.Spec.Config.Command
	if len(command) == 0 {
		return errors.New("no command specified in task")
	}

	// Convertir el mapa de variables de entorno
	envVars := t.Spec.Config.Environment

	// Ejecutar el comando a través del cliente gRPC
	err := p.workerClient.ExecuteCommand(
		t.Metadata.ID.String(),
		command,
		t.Spec.Config.WorkingDir,
		envVars,
	)
	if err != nil {
		p.logger.Error("failed to execute command",
			"task_id", t.Metadata.ID,
			"error", err)
		t.UpdateStatus(task.TaskPhaseFailed, t.Metadata.ID.String(), err.Error())
		return errors.Wrap(err, "failed to execute command")
	}

	t.UpdateStatus(task.TaskPhaseRunning, t.Metadata.ID.String(), "Task sent to worker")
	return nil
}

// GetTaskStatus implementa ResourcePool.GetTaskStatus
func (p *DockerHostPool) GetTaskStatus(ctx context.Context, taskID string) (*resource.TaskStatus, error) {
	// TODO: Implementar la llamada gRPC al manager para obtener el estado de la tarea
	return &resource.TaskStatus{
		ID:    taskID,
		State: resource.TaskStateRunning,
	}, nil
}

// GetTaskLogs implementa ResourcePool.GetTaskLogs
func (p *DockerHostPool) GetTaskLogs(ctx context.Context, taskID string, follow bool) (io.ReadCloser, error) {
	// TODO: Implementar la obtención de logs a través del manager
	return nil, errors.New("not implemented")
}

// StopTask implementa ResourcePool.StopTask
func (p *DockerHostPool) StopTask(ctx context.Context, taskID string) error {
	// TODO: Implementar la llamada gRPC al manager para detener la tarea
	return errors.New("not implemented")
}

// Cleanup implementa ResourcePool.Cleanup
func (p *DockerHostPool) Cleanup(ctx context.Context) error {
	// Detener y eliminar workers
	for _, workerID := range p.workerIDs {
		if err := p.client.ContainerRemove(ctx, workerID, container.RemoveOptions{
			Force:         true,
			RemoveVolumes: true,
		}); err != nil {
			p.logger.Error("failed to remove worker container", "container_id", workerID, "error", err)
		}
	}

	// Detener y eliminar manager
	if err := p.client.ContainerRemove(ctx, p.managerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		p.logger.Error("failed to remove manager container", "error", err)
	}

	// Eliminar red
	if err := p.client.NetworkRemove(ctx, p.networkID); err != nil {
		p.logger.Error("failed to remove network", "error", err)
	}

	return nil
}
