package interfaces

import "dev.rubentxu.devops-platform/core/domain"

// WorkerRepository define la interfaz para persistir y recuperar workers
type WorkerRepository interface {
	Save(worker domain.Worker) error
	Get(workerID string) (domain.Worker, error)
	Update(worker domain.Worker) error
	Delete(workerID string) error
	GetAvailable() (domain.Worker, error)
}
