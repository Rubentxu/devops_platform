package client

import (
	"context"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"

	"log"

	"google.golang.org/grpc"
)

// ManagerClient encapsula la funcionalidad del cliente gRPC para el WorkerManager
type ManagerClient struct {
	client manager.WorkerRegistrationServiceClient
}

// NewManagerClient crea una nueva instancia del cliente
func NewManagerClient(conn *grpc.ClientConn) *ManagerClient {
	return &ManagerClient{
		client: manager.NewWorkerRegistrationServiceClient(conn),
	}
}

// RegisterWorker registra un nuevo worker en el manager
func (c *ManagerClient) RegisterWorker(workerID, workerName, ip string) error {
	req := &manager.RegisterWorkerRequest{
		WorkerId:   workerID,
		WorkerName: workerName,
		Ip:         ip,
	}

	resp, err := c.client.RegisterWorker(context.Background(), req)
	if err != nil {
		return err
	}

	if !resp.Success {
		log.Printf("Fallo al registrar Worker: %s", resp.Message)
		return nil
	}

	log.Printf("Worker registrado correctamente")
	return nil
}
