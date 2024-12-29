package usecase

import (
	"dev.rubentxu.devops-platform/core/domain"
)

// WorkerUseCase define métodos para registrar y consultar workers.
type WorkerUseCase interface {
	RegisterWorker(worker domain.Worker) error
	GetAvailableWorker() (domain.Worker, error)
}

// Implementación mínima.
type workerUseCaseImpl struct {
	workers map[string]domain.Worker
}

func NewWorkerUseCaseImpl() WorkerUseCase {
	return &workerUseCaseImpl{
		workers: make(map[string]domain.Worker),
	}
}

func (wc *workerUseCaseImpl) RegisterWorker(worker domain.Worker) error {
	wc.workers[worker.ID] = worker
	return nil
}

func (wc *workerUseCaseImpl) GetAvailableWorker() (domain.Worker, error) {
	// Lógica simple: retornar el primer worker no ocupado.
	for _, w := range wc.workers {
		if !w.IsBusy && w.IsHealthy {
			return w, nil
		}
	}
	return domain.Worker{}, nil
}
