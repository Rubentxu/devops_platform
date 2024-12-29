package task

import (
	"time"

	"github.com/google/uuid"
)

// TaskSpec define la especificación de una tarea
type TaskSpec struct {
	Name      string           `json:"name"`
	Image     string           `json:"image"`
	Resources ResourceRequests `json:"resources"`
	Network   NetworkConfig    `json:"network,omitempty"`
	Storage   StorageConfig    `json:"storage,omitempty"`
	Config    ContainerConfig  `json:"config"`
}

// ResourceRequests especifica los recursos necesarios para la tarea
type ResourceRequests struct {
	CPU    float64 `json:"cpu"`    // Unidades de CPU (1.0 = 1 core)
	Memory int64   `json:"memory"` // Memoria en bytes
	Disk   int64   `json:"disk"`   // Almacenamiento en bytes
}

// NetworkConfig define la configuración de red
type NetworkConfig struct {
	ExposedPorts map[string]struct{} `json:"exposed_ports,omitempty"` // puertos expuestos
	PortBindings map[string]string   `json:"port_bindings,omitempty"` // mapeo de puertos host:container
}

// StorageConfig define la configuración de almacenamiento
type StorageConfig struct {
	Volumes map[string]string `json:"volumes,omitempty"` // path_host:path_container
}

// ContainerConfig define la configuración del contenedor
type ContainerConfig struct {
	RestartPolicy string            `json:"restart_policy"`
	Environment   map[string]string `json:"environment,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	WorkingDir    string            `json:"working_dir,omitempty"`
	Command       []string          `json:"command,omitempty"`
	Args          []string          `json:"args,omitempty"`
}

// TaskStatus representa el estado actual de una tarea
type TaskStatus struct {
	State           State      `json:"state"`
	ContainerID     string     `json:"container_id,omitempty"`
	StartTime       *time.Time `json:"start_time,omitempty"`
	FinishTime      *time.Time `json:"finish_time,omitempty"`
	Message         string     `json:"message,omitempty"`
	Reason          string     `json:"reason,omitempty"`
	ExitCode        *int32     `json:"exit_code,omitempty"`
	RestartCount    int32      `json:"restart_count"`
	LastRestartTime *time.Time `json:"last_restart_time,omitempty"`
}

// TaskMetadata contiene metadatos comunes
type TaskMetadata struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Task representa una tarea completa con su especificación y estado
type Task struct {
	Metadata TaskMetadata `json:"metadata"`
	Spec     TaskSpec     `json:"spec"`
	Status   TaskStatus   `json:"status"`
}

// NewTask crea una nueva tarea con valores por defecto
func NewTask(name string, image string) *Task {
	now := time.Now()
	return &Task{
		Metadata: TaskMetadata{
			ID:        uuid.New(),
			Name:      name,
			CreatedAt: now,
			Labels:    make(map[string]string),
		},
		Spec: TaskSpec{
			Name:  name,
			Image: image,
			Resources: ResourceRequests{
				CPU:    0.1,
				Memory: 256 * 1024 * 1024,  // 256MB
				Disk:   1024 * 1024 * 1024, // 1GB
			},
			Network: NetworkConfig{
				ExposedPorts: make(map[string]struct{}),
				PortBindings: make(map[string]string),
			},
			Storage: StorageConfig{
				Volumes: make(map[string]string),
			},
			Config: ContainerConfig{
				RestartPolicy: "no",
				Environment:   make(map[string]string),
				Labels:        make(map[string]string),
			},
		},
		Status: TaskStatus{
			State: Pending,
		},
	}
}

// WithResources configura los recursos de la tarea
func (t *Task) WithResources(cpu float64, memory int64, disk int64) *Task {
	t.Spec.Resources = ResourceRequests{
		CPU:    cpu,
		Memory: memory,
		Disk:   disk,
	}
	return t
}

// WithNetwork configura la red de la tarea
func (t *Task) WithNetwork(exposedPorts map[string]struct{}, portBindings map[string]string) *Task {
	t.Spec.Network = NetworkConfig{
		ExposedPorts: exposedPorts,
		PortBindings: portBindings,
	}
	return t
}

// WithStorage configura el almacenamiento de la tarea
func (t *Task) WithStorage(volumes map[string]string) *Task {
	t.Spec.Storage = StorageConfig{
		Volumes: volumes,
	}
	return t
}

// WithConfig configura los parámetros del contenedor
func (t *Task) WithConfig(restartPolicy string, env map[string]string, labels map[string]string) *Task {
	t.Spec.Config = ContainerConfig{
		RestartPolicy: restartPolicy,
		Environment:   env,
		Labels:        labels,
	}
	return t
}

// UpdateStatus actualiza el estado de la tarea
func (t *Task) UpdateStatus(state State, containerID string, message string) {
	now := time.Now()
	t.Status.State = state
	t.Status.ContainerID = containerID
	t.Status.Message = message

	switch state {
	case Running:
		if t.Status.StartTime == nil {
			t.Status.StartTime = &now
		}
	case Completed, Failed:
		t.Status.FinishTime = &now
	}
}

// IsComplete verifica si la tarea ha terminado
func (t *Task) IsComplete() bool {
	return t.Status.State == Completed || t.Status.State == Failed
}

// Duration retorna la duración de la tarea
func (t *Task) Duration() time.Duration {
	if t.Status.StartTime == nil {
		return 0
	}
	if t.Status.FinishTime != nil {
		return t.Status.FinishTime.Sub(*t.Status.StartTime)
	}
	return time.Since(*t.Status.StartTime)
}

type TaskEvent struct {
	ID        uuid.UUID
	State     State
	Timestamp time.Time
	Task      Task
}

type TaskResult struct {
	Error       error
	Action      string
	ContainerId string
	Result      string
}

func NewConfig(t *Task) ContainerConfig {
	return ContainerConfig{
		RestartPolicy: "no",
		Environment:   t.Spec.Config.Environment,
		Labels:        t.Spec.Config.Labels,
		WorkingDir:    t.Spec.Config.WorkingDir,
		Command:       t.Spec.Config.Command,
		Args:          t.Spec.Config.Args,
	}
}
