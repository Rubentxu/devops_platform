package server

import (
	"context"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"
	w "dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/worker"
	"dev.rubentxu.devops-platform/core/domain"
	"dev.rubentxu.devops-platform/core/usecase"
	"fmt"

	"google.golang.org/grpc"
)

type workerRegistrationServer struct {
	manager.UnimplementedWorkerRegistrationServiceServer
	workerUseCase usecase.WorkerUseCase
}

func NewWorkerRegistrationServer(uc usecase.WorkerUseCase) *workerRegistrationServer {
	return &workerRegistrationServer{
		workerUseCase: uc,
	}
}

func (s *workerRegistrationServer) RegisterWorker(ctx context.Context, req *manager.RegisterWorkerRequest) (*manager.RegisterWorkerResponse, error) {
	worker := domain.Worker{
		ID:        req.WorkerId,
		Name:      req.WorkerName,
		IP:        req.Ip,
		IsHealthy: true,
		IsBusy:    false,
	}
	err := s.workerUseCase.RegisterWorker(worker)
	if err != nil {
		return &manager.RegisterWorkerResponse{
			Success: false,
			Message: err.Error(),
		}, err
	}
	return &manager.RegisterWorkerResponse{
		Success: true,
		Message: "Worker registrado correctamente",
	}, nil
}

type processManagementServer struct {
	manager.UnimplementedProcessManagementServiceServer
	processUseCase usecase.ProcessUseCase
	workerUseCase  usecase.WorkerUseCase
}

func NewProcessManagementServer(puc usecase.ProcessUseCase, wuc usecase.WorkerUseCase) *processManagementServer {
	return &processManagementServer{
		processUseCase: puc,
		workerUseCase:  wuc,
	}
}

func (p *processManagementServer) ExecuteDistributedCommand(req *manager.ExecuteCommandRequest, stream manager.ProcessManagementService_ExecuteDistributedCommandServer) error {
	// 1. Programar el proceso en el Manager (estado = Pending).
	_, err := p.processUseCase.ScheduleProcess(domain.ProcessRequest{
		ID:         req.ProcessId,
		Command:    req.Command,
		WorkingDir: req.WorkingDir,
		EnvVars:    req.EnvVars,
	})
	if err != nil {
		return err
	}

	// 2. Escoger worker disponible
	worker, err := p.workerUseCase.GetAvailableWorker()
	if err != nil || worker.ID == "" {
		return fmt.Errorf("no hay workers disponibles")
	}

	// 3. Conectar con el worker y enviar el comando
	conn, err := grpc.Dial(fmt.Sprintf("%s:50052", worker.IP), grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("error conectando con worker: %v", err)
	}
	defer conn.Close()

	workerClient := w.NewWorkerProcessServiceClient(conn)
	workerStream, err := workerClient.ExecuteCommand(context.Background())
	if err != nil {
		return fmt.Errorf("error iniciando stream con worker: %v", err)
	}

	// 4. Enviar comando al worker
	if err := workerStream.Send(&w.ExecuteCommandRequest{
		ProcessId:  req.ProcessId,
		Command:    req.Command,
		WorkingDir: req.WorkingDir,
		EnvVars:    req.EnvVars,
	}); err != nil {
		return fmt.Errorf("error enviando comando al worker: %v", err)
	}

	// 5. Recibir y reenviar la salida del worker
	for {
		workerOutput, err := workerStream.Recv()
		if err != nil {
			_ = p.processUseCase.UpdateProcessState(req.ProcessId, domain.StateCompleted, 0, "")
			return nil
		}

		managerOutput := &manager.CommandOutput{
			ProcessId: workerOutput.ProcessId,
			IsStderr:  workerOutput.IsStderr,
			Content:   workerOutput.Content,
			State:     workerOutput.State,
			ExitCode:  workerOutput.ExitCode,
			ErrorMsg:  workerOutput.ErrorMsg,
		}

		if err := stream.Send(managerOutput); err != nil {
			return err
		}

		if workerOutput.State == "COMPLETED" || workerOutput.State == "FAILED" {
			_ = p.processUseCase.UpdateProcessState(req.ProcessId, domain.ProcessState(workerOutput.State), workerOutput.ExitCode, workerOutput.ErrorMsg)
			break
		}
	}

	return nil
}

func (p *processManagementServer) TerminateProcess(ctx context.Context, req *manager.TerminateProcessRequest) (*manager.TerminateProcessResponse, error) {
	// Aquí invocaríamos al worker vía gRPC para matar el proceso.
	// Ejemplo simplificado:
	_ = p.processUseCase.UpdateProcessState(req.ProcessId, domain.StateAborted, 0, "Terminado manualmente")
	return &manager.TerminateProcessResponse{
		Success: true,
		Message: "Proceso terminado",
	}, nil
}

func (p *processManagementServer) GetProcessStatus(ctx context.Context, req *manager.GetProcessStatusRequest) (*manager.GetProcessStatusResponse, error) {
	// Ejemplo: usar un método de consulta (no implementado en el snippet)
	// Supongamos que tenemos una forma de obtener el status
	// status := p.processUseCase.GetProcessStatus(req.ProcessId)
	return &manager.GetProcessStatusResponse{
		ProcessId: req.ProcessId,
		State:     "UNKNOWN",
		ExitCode:  -1,
		ErrorMsg:  "No implementado",
	}, nil
}
