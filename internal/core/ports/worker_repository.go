package ports

import "dev.rubentxu.devops-platform/core/domain"

// WorkerRepository define la interfaz para persistir y recuperar workers
type WorkerRepository interface {
	Save(worker domain.WorkerInstance) error
	Get(workerID string) (domain.WorkerInstance, error)
	Update(worker domain.WorkerInstance) error
	Delete(workerID string) error
	GetAvailable() (domain.WorkerInstance, error)
}
