package main

import (
	"context"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/worker"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/server"

	"fmt"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
)

func main() {
	// 1. Conectarse al WorkerManager para hacer RegisterWorker (simulando).
	go registerToManager()

	// 2. Iniciar servidor gRPC local del Worker
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Error al iniciar listener worker: %v", err)
	}

	grpcServer := grpc.NewServer()
	worker.RegisterWorkerProcessServiceServer(grpcServer, &server.WorkerServer{})
	log.Println("Worker escuchando en :50052...")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Fallo al servir gRPC Worker: %v", err)
	}
}

func registerToManager() {
	maxRetries := 5
	retryDelay := 2 * time.Second

	for i := 0; i < maxRetries; i++ {
		if err := tryRegisterToManager(); err != nil {
			log.Printf("Intento %d de %d fallido: %v", i+1, maxRetries, err)
			time.Sleep(retryDelay)
			continue
		}
		return
	}
	log.Printf("No se pudo registrar el worker después de %d intentos", maxRetries)
}

func tryRegisterToManager() error {
	managerHost := os.Getenv("MANAGER_HOST")
	if managerHost == "" {
		managerHost = "localhost"
	}

	managerPort := os.Getenv("MANAGER_PORT")
	if managerPort == "" {
		managerPort = "50051"
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "worker-test"
	}

	managerAddr := fmt.Sprintf("%s:%s", managerHost, managerPort)
	conn, err := grpc.Dial(managerAddr, grpc.WithInsecure())
	if err != nil {
		log.Printf("No se pudo conectar al manager en %s: %v", managerAddr, err)
		return err
	}
	defer conn.Close()

	client := manager.NewWorkerRegistrationServiceClient(conn)
	req := &manager.RegisterWorkerRequest{
		WorkerId:   hostname,
		WorkerName: hostname,
		Ip:         hostname,
	}

	resp, err := client.RegisterWorker(context.Background(), req)
	if err != nil {
		log.Printf("Error al registrar Worker: %v", err)
		return err
	}
	if !resp.Success {
		log.Printf("Fallo al registrar Worker: %s", resp.Message)
		return fmt.Errorf("fallo al registrar worker: %s", resp.Message)
	}
	log.Printf("Worker registrado correctamente en el Manager en %s", managerAddr)
	return nil
}
