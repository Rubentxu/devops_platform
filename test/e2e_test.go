package test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	mgr "dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"
	"github.com/stretchr/testify/require"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func setupTest(t *testing.T) (context.Context, string, func()) {
	// Limpiar cualquier contenedor anterior que pudiera haber quedado
	compose, err := tc.NewDockerCompose("docker-compose.yaml")
	require.NoError(t, err, "NewDockerCompose()")

	err = compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal)
	if err != nil {
		t.Logf("Warning during initial cleanup: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	// Función para mostrar los logs de un contenedor
	showContainerLogs := func(service string) {
		container, err := compose.ServiceContainer(ctx, service)
		if err != nil {
			t.Logf("Error getting %s container: %v", service, err)
			return
		}
		logs, err := container.Logs(ctx)
		if err != nil {
			t.Logf("Error getting %s logs: %v", service, err)
			return
		}
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			t.Logf("%s: %s", service, scanner.Text())
		}
	}

	// Mostrar los logs mientras se inician los servicios
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				showContainerLogs("manager")
				showContainerLogs("worker1")
				time.Sleep(time.Second)
			}
		}
	}()

	// Iniciar los servicios
	t.Log("Starting services...")
	err = compose.Up(ctx)
	require.NoError(t, err, "compose.Up()")
	t.Log("Services started successfully")

	// Dar tiempo inicial para que los servicios se inicien
	time.Sleep(5 * time.Second)

	// Verificar que los servicios están realmente disponibles
	managerAddr := "localhost:50051"

	// Intentar conectar con el manager
	t.Log("Checking manager availability...")
	err = waitForGRPCService(ctx, managerAddr)
	require.NoError(t, err, "Manager service is not available")
	t.Log("Manager is available")

	// Esperar a que los workers se registren
	t.Log("Waiting for workers to register...")
	err = waitForWorkers(ctx, managerAddr)
	require.NoError(t, err, "Workers not registered")
	t.Log("Workers registered successfully")

	cleanup := func() {
		if err := compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal); err != nil {
			t.Logf("Error during cleanup: %v", err)
		}
	}

	return ctx, managerAddr, cleanup
}

func waitForGRPCService(ctx context.Context, addr string) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(2*time.Second))
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("service at %s not available after timeout", addr)
}

func waitForWorkers(ctx context.Context, managerAddr string) error {
	deadline := time.Now().Add(30 * time.Second)
	conn, err := grpc.NewClient(managerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to manager: %v", err)
	}
	defer conn.Close()

	client := mgr.NewProcessManagementServiceClient(conn)

	for time.Now().Before(deadline) {
		// Intentar ejecutar un comando simple para verificar si hay workers disponibles
		req := &mgr.ExecuteCommandRequest{
			ProcessId:  "test-worker-availability",
			Command:    []string{"echo", "test"},
			WorkingDir: "/tmp",
		}

		stream, err := client.ExecuteDistributedCommand(ctx, req)
		if err == nil {
			// Leer la respuesta para asegurarnos de que todo está bien
			_, err = stream.Recv()
			if err == nil || err == io.EOF {
				return nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("no workers available after timeout")
}

func TestE2E(t *testing.T) {
	ctx, managerAddr, cleanup := setupTest(t)
	defer cleanup()

	// Connect to Manager via gRPC
	conn, err := grpc.NewClient(managerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	managerClient := mgr.NewProcessManagementServiceClient(conn)

	// Execute a command that generates multiple traces and takes some time
	processID := "test-process-123"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "for i in {1..5}; do echo 'Simulando salida'; sleep 1; done; echo 'Fin proceso'; exit 0"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	stream, err := managerClient.ExecuteDistributedCommand(ctx, req)
	require.NoError(t, err)

	// Read logs by streaming and verify that we get the expected outputs
	var lines []string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			require.NoError(t, err, "Error receiving stream")
			break
		}
		fmt.Printf("[LOG stream] %s\n", msg.Content)
		lines = append(lines, msg.Content)
	}

	require.Contains(t, lines, "Fin proceso", "Expected logs not received")

	t.Log("E2E TEST completed successfully.")
}
