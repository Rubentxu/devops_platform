package main

import (
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/server"
	"dev.rubentxu.devops-platform/core/usecase"
	"log"
	"net"

	"google.golang.org/grpc"
)

func main() {
	// 1. Inicializar casos de uso
	workerUC := usecase.NewWorkerUseCaseImpl()
	processUC := usecase.NewProcessUseCaseImpl()

	// 2. Crear instancias de servidores gRPC
	workerRegServer := server.NewWorkerRegistrationServer(workerUC)
	procMgmtServer := server.NewProcessManagementServer(processUC, workerUC)

	// 3. Iniciar servidor gRPC
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Error al iniciar listener: %v", err)
	}

	grpcServer := grpc.NewServer()
	manager.RegisterWorkerRegistrationServiceServer(grpcServer, workerRegServer)
	manager.RegisterProcessManagementServiceServer(grpcServer, procMgmtServer)

	log.Printf("Worker Manager escuchando en :50051...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Fallo al servir gRPC: %v", err)
	}
}
