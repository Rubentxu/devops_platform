package ports

import (
	"context"
	"dev.rubentxu.devops-platform/core/domain/resource"
	"dev.rubentxu.devops-platform/core/domain/task"
	"io"
)

// ResourcePool define la interfaz para un pool de recursos
type ResourcePool interface {
	// GetID retorna el identificador único del pool
	GetID() string

	// GetStats retorna las estadísticas actuales de recursos
	GetStats() resource.ResourceStats

	// GetHealthStatus retorna el estado de salud actual
	GetHealthStatus() resource.HealthStatus

	// ExecuteTask ejecuta una tarea en el pool de recursos
	ExecuteTask(ctx context.Context, task *task.Task) error

	// GetTaskStatus obtiene el estado actual de una tarea
	GetTaskStatus(ctx context.Context, taskID string) (*resource.TaskStatus, error)

	// GetTaskLogs obtiene los logs de una tarea
	GetTaskLogs(ctx context.Context, taskID string, follow bool) (io.ReadCloser, error)

	// StopTask detiene una tarea en ejecución
	StopTask(ctx context.Context, taskID string) error

	// Cleanup realiza la limpieza de recursos
	Cleanup(ctx context.Context) error
}
