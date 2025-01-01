package server

import (
	"context"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"
	"dev.rubentxu.devops-platform/adapters/grpc/worker"

	"dev.rubentxu.devops-platform/core/domain"
)

type workerRegistrationServer struct {
	manager.UnimplementedWorkerRegistrationServiceServer
	workerService worker.WorkerService
}

func NewWorkerRegistrationServer(uc worker.WorkerService) *workerRegistrationServer {
	return &workerRegistrationServer{
		workerService: uc,
	}
}

func (s *workerRegistrationServer) RegisterWorker(ctx context.Context, req *manager.RegisterWorkerRequest) (*manager.RegisterWorkerResponse, error) {
	worker := domain.WorkerInstance{
		ID:        req.WorkerId,
		Name:      req.WorkerName,
		Address:   req.Address,
		Port:      req.Port,
		IsHealthy: true,
		IsBusy:    false,
	}
	err := s.workerService.RegisterWorker(worker)
	if err != nil {
		return &manager.RegisterWorkerResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}
	return &manager.RegisterWorkerResponse{
		Success: true,
		Message: "WorkerInstance registrado correctamente",
	}, nil
}
