package task

import (
	"time"

	"dev.rubentxu.devops-platform/core/domain"
	"github.com/google/uuid"
)

// TaskSpec define la especificación de una tarea
type TaskSpec struct {
	SandboxConfig domain.SandboxConfig
	Command       []string
	Timeout       time.Duration
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
			SandboxConfig: domain.SandboxConfig{
				Image: image,
				Resources: domain.ResourceRequests{
					CPU:    "0.1",
					Memory: "256MB",
					Disk:   "1GB",
				},
				Network: domain.NetworkConfig{
					ExposedPorts: make(map[string]struct{}),
					PortBindings: make(map[string]string),
				},
				Storage: domain.StorageConfig{
					Volumes: make(map[string]string),
				},
			},
		},
		Status: TaskStatus{
			State: Pending,
		},
	}
}

// WithResources configura los recursos de la tarea
func (t *Task) WithResources(cpu string, memory string, disk string) *Task {
	t.Spec.SandboxConfig.Resources = domain.ResourceRequests{
		CPU:    cpu,
		Memory: memory,
		Disk:   disk,
	}
	return t
}

// WithNetwork configura la red de la tarea
func (t *Task) WithNetwork(exposedPorts map[string]struct{}, portBindings map[string]string) *Task {
	t.Spec.SandboxConfig.Network = domain.NetworkConfig{
		ExposedPorts: exposedPorts,
		PortBindings: portBindings,
	}
	return t
}

// WithStorage configura el almacenamiento de la tarea
func (t *Task) WithStorage(volumes map[string]string) *Task {
	t.Spec.SandboxConfig.Storage = domain.StorageConfig{
		Volumes: volumes,
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

func (t Task) Validate() bool {
	return t.Metadata.ID != uuid.Nil &&
		len(t.Spec.Command) > 0 &&
		t.Spec.Timeout > 0
}
