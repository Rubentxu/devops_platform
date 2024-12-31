package resource

import (
	"context"
	"github.com/c9s/goprocinfo/linux"
	"runtime"
	"time"
)

type Stats struct {
	MemStats  *linux.MemInfo
	DiskStats *linux.Disk
	CpuStats  *linux.CPUStat
	LoadStats *linux.LoadAvg
	TaskCount int
}

// ResourceStats representa las estadísticas de recursos del sistema
type ResourceStats struct {
	CPUStats    CPUStats    `json:"cpu_stats"`
	MemoryStats MemoryStats `json:"memory_stats"`
	DiskStats   DiskStats   `json:"disk_stats"`
}

// CPUStats representa las estadísticas de CPU
type CPUStats struct {
	UsagePercentage float64 `json:"usage_percentage"`
	AvailableCores  int     `json:"available_cores"`
}

// NewCPUStats crea una nueva instancia de CPUStats con valores por defecto
func NewCPUStats() CPUStats {
	return CPUStats{
		UsagePercentage: 0.0,
		AvailableCores:  runtime.NumCPU(),
	}
}

// MemoryStats representa las estadísticas de memoria
type MemoryStats struct {
	TotalKb uint64 `json:"total_kb"`
	UsedKb  uint64 `json:"used_kb"`
	FreeKb  uint64 `json:"free_kb"`
}

// NewMemoryStats crea una nueva instancia de MemoryStats con valores por defecto
func NewMemoryStats() MemoryStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return MemoryStats{
		TotalKb: m.Sys / 1024,
		UsedKb:  (m.Sys - m.HeapIdle) / 1024,
		FreeKb:  m.HeapIdle / 1024,
	}
}

// DiskStats representa las estadísticas de disco
type DiskStats struct {
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
}

// HealthStatus representa el estado de salud del recurso
type HealthStatus struct {
	IsHealthy bool      `json:"is_healthy"`
	LastCheck time.Time `json:"last_check"`
	Message   string    `json:"message,omitempty"`
}

// NewHealthStatus crea una nueva instancia de HealthStatus con valores por defecto
func NewHealthStatus() HealthStatus {
	return HealthStatus{
		IsHealthy: true,
		LastCheck: time.Now(),
	}
}

// TaskState representa los posibles estados de una tarea
type TaskState string

const (
	TaskStatePending   TaskState = "PENDING"
	TaskStateRunning   TaskState = "RUNNING"
	TaskStateCompleted TaskState = "COMPLETED"
	TaskStateFailed    TaskState = "FAILED"
)

// TaskStatus representa el estado actual de una tarea
type TaskStatus struct {
	ID         string     `json:"id"`
	State      TaskState  `json:"state"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	FinishTime *time.Time `json:"finish_time,omitempty"`
	ExitCode   *int32     `json:"exit_code,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// ResourcePool define la interfaz para un pool de recursos
type ResourcePool interface {
	// GetID retorna el identificador único del pool
	GetID() string

	// GetStats retorna las estadísticas actuales de recursos
	GetStats() ResourceStats

	CreateWorker(ctx context.Context, workerID string) error
}

// NewResourceStats crea una nueva instancia de ResourceStats con valores por defecto
func NewResourceStats() ResourceStats {
	return ResourceStats{
		CPUStats:    NewCPUStats(),
		MemoryStats: NewMemoryStats(),
		DiskStats:   DiskStats{}, // Los valores de disco necesitarán ser obtenidos del sistema
	}
}
