package usecase

import (
	"dev.rubentxu.devops-platform/core/domain"
	"log"
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
	log.Printf("Worker registrado: ID=%s, Name=%s, IP=%s, IsHealthy=%t, IsBusy=%t", worker.ID, worker.Name, worker.IP, worker.IsHealthy, worker.IsBusy)
	return nil
}

func (wc *workerUseCaseImpl) GetAvailableWorker() (domain.Worker, error) {
	// Lógica simple: retornar el primer worker no ocupado.
	for _, w := range wc.workers {
		log.Printf("Verificando worker: ID=%s, IsHealthy=%t, IsBusy=%t", w.ID, w.IsHealthy, w.IsBusy)
		if !w.IsBusy && w.IsHealthy {
			log.Printf("Worker disponible encontrado: ID=%s", w.ID)
			return w, nil
		}
	}
	log.Println("No hay workers disponibles")
	return domain.Worker{}, nil
}
