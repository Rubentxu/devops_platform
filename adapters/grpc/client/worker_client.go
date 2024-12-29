package client

import (
	"context"
	"dev.rubentxu.devops-platform/adapters/grpc/protos/worker"

	"google.golang.org/grpc"
)

// WorkerClient encapsula la funcionalidad del cliente gRPC para el Worker
type WorkerClient struct {
	client worker.WorkerProcessServiceClient
}

// NewWorkerClient crea una nueva instancia del cliente
func NewWorkerClient(conn *grpc.ClientConn) *WorkerClient {
	return &WorkerClient{
		client: worker.NewWorkerProcessServiceClient(conn),
	}
}

// ExecuteCommand ejecuta un comando en el worker
func (c *WorkerClient) ExecuteCommand(processID string, command []string, workingDir string, envVars map[string]string) error {
	stream, err := c.client.ExecuteCommand(context.Background())
	if err != nil {
		return err
	}

	// Enviar la solicitud
	req := &worker.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    command,
		WorkingDir: workingDir,
		EnvVars:    envVars,
	}

	if err := stream.Send(req); err != nil {
		return err
	}

	// Recibir respuestas
	for {
		resp, err := stream.Recv()
		if err != nil {
			return err
		}

		// Procesar la respuesta según sea necesario
		_ = resp
	}
}
