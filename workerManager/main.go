package main

import (
	"dev.rubentxu.devops-platform/adapters/grpc/protos/manager"
	"dev.rubentxu.devops-platform/adapters/grpc/server"
	"dev.rubentxu.devops-platform/adapters/worker"
	"log"
	"net"

	"google.golang.org/grpc"
)

func main() {
	// 1. Inicializar casos de uso

	processUC := worker.NewWorkerService()

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
