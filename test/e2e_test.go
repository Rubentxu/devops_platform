package test

//
//import (
//	"bufio"
//	"context"
//	"fmt"
//	"io"
//	"strings"
//	"testing"
//	"time"
//
//	mgr "dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"
//	"github.com/stretchr/testify/require"
//	tc "github.com/testcontainers/testcontainers-go/modules/compose"
//	"google.golang.org/grpc"
//	"google.golang.org/grpc/credentials/insecure"
//)
//
//func setupTest(t *testing.T) (context.Context, string, func()) {
//	// Limpiar cualquier contenedor anterior que pudiera haber quedado
//	compose, err := tc.NewDockerCompose("docker-compose.yaml")
//	require.NoError(t, err, "NewDockerCompose()")
//
//	err = compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal)
//	if err != nil {
//		t.Logf("Warning during initial cleanup: %v", err)
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
//	t.Cleanup(cancel)
//
//	// Función para mostrar los logs de un contenedor
//	showContainerLogs := func(service string) {
//		container, err := compose.ServiceContainer(ctx, service)
//		if err != nil {
//			t.Logf("Error getting %s container: %v", service, err)
//			return
//		}
//		logs, err := container.Logs(ctx)
//		if err != nil {
//			t.Logf("Error getting %s logs: %v", service, err)
//			return
//		}
//		defer logs.Close()
//		scanner := bufio.NewScanner(logs)
//		for scanner.Scan() {
//			t.Logf("%s: %s", service, scanner.Text())
//		}
//	}
//
//	// Mostrar los logs mientras se inician los servicios
//	go func() {
//		for {
//			select {
//			case <-ctx.Done():
//				return
//			default:
//				showContainerLogs("manager")
//				showContainerLogs("worker1")
//				time.Sleep(time.Second)
//			}
//		}
//	}()
//
//	// Iniciar los servicios
//	t.Log("Starting services...")
//	err = compose.Up(ctx)
//	require.NoError(t, err, "compose.Up()")
//	t.Log("Services started successfully")
//
//	// Dar tiempo inicial para que los servicios se inicien
//	time.Sleep(5 * time.Second)
//
//	// Verificar que los servicios están realmente disponibles
//	managerAddr := "localhost:50051"
//
//	// Intentar conectar con el manager
//	t.Log("Checking manager availability...")
//	err = waitForGRPCService(ctx, managerAddr)
//	require.NoError(t, err, "Manager service is not available")
//	t.Log("Manager is available")
//
//	// Esperar a que los workers se registren
//	t.Log("Waiting for workers to register...")
//	err = waitForWorkers(ctx, managerAddr)
//	require.NoError(t, err, "Workers not registered")
//	t.Log("Workers registered successfully")
//
//	cleanup := func() {
//		if err := compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal); err != nil {
//			t.Logf("Error during cleanup: %v", err)
//		}
//	}
//
//	return ctx, managerAddr, cleanup
//}
//
//func waitForGRPCService(ctx context.Context, addr string) error {
//	deadline := time.Now().Add(30 * time.Second)
//	for time.Now().Before(deadline) {
//		conn, err := grpc.NewClient(addr,
//			grpc.WithTransportCredentials(insecure.NewCredentials()),
//			grpc.WithBlock(),
//			grpc.WithTimeout(2*time.Second))
//		if err == nil {
//			conn.Close()
//			return nil
//		}
//		time.Sleep(2 * time.Second)
//	}
//	return fmt.Errorf("service at %s not available after timeout", addr)
//}
//
//func waitForWorkers(ctx context.Context, managerAddr string) error {
//	deadline := time.Now().Add(30 * time.Second)
//	conn, err := grpc.NewClient(managerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
//	if err != nil {
//		return fmt.Errorf("failed to connect to manager: %v", err)
//	}
//	defer conn.Close()
//
//	client := mgr.NewProcessManagementServiceClient(conn)
//
//	for time.Now().Before(deadline) {
//		// Intentar ejecutar un comando simple para verificar si hay workers disponibles
//		req := &mgr.ExecuteCommandRequest{
//			ProcessId:  "test-worker-availability",
//			Command:    []string{"echo", "test"},
//			WorkingDir: "/tmp",
//		}
//
//		stream, err := client.ExecuteDistributedCommand(ctx, req)
//		if err == nil {
//			// Leer la respuesta para asegurarnos de que todo está bien
//			_, err = stream.Recv()
//			if err == nil || err == io.EOF {
//				return nil
//			}
//		}
//		time.Sleep(2 * time.Second)
//	}
//	return fmt.Errorf("no workers available after timeout")
//}
//
//func TestE2E(t *testing.T) {
//	ctx, managerAddr, cleanup := setupTest(t)
//	defer cleanup()
//
//	// Connect to Manager via gRPC
//	conn, err := grpc.Dial(managerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
//	require.NoError(t, err)
//	defer conn.Close()
//
//	managerClient := mgr.NewProcessManagementServiceClient(conn)
//
//	// Test case 1: Comando básico
//	t.Run("Basic Command", func(t *testing.T) {
//		processID := "test-basic-cmd"
//	req := &mgr.ExecuteCommandRequest{
//		ProcessId:  processID,
//			Command:    []string{"echo", "Hello World"},
//		WorkingDir: "/tmp",
//	}
//
//	stream, err := managerClient.ExecuteDistributedCommand(ctx, req)
//	require.NoError(t, err)
//
//	var lines []string
//	for {
//		msg, err := stream.Recv()
//			if err == io.EOF {
//				break
//			}
//			require.NoError(t, err)
//			t.Logf("Received output: %s", msg.Content)
//			lines = append(lines, msg.Content)
//		}
//
//		t.Logf("All received output: %v", lines)
//		require.Greater(t, len(lines), 0, "Expected at least one line of output")
//		found := false
//		for _, line := range lines {
//			if strings.Contains(line, "Hello World") {
//				found = true
//				break
//			}
//		}
//		require.True(t, found, "Expected output to contain 'Hello World'")
//	})
//
//	// Test case 2: Comando con variables de entorno
//	t.Run("Environment Variables", func(t *testing.T) {
//		req := &mgr.ExecuteCommandRequest{
//			ProcessId:  "test-env-cmd",
//			Command:    []string{"sh", "-c", "echo $TEST_VAR"},
//			WorkingDir: "/tmp",
//			EnvVars: map[string]string{
//				"TEST_VAR": "test value",
//			},
//		}
//
//		stream, err := managerClient.ExecuteDistributedCommand(ctx, req)
//		require.NoError(t, err)
//
//		var lines []string
//		for {
//			msg, err := stream.Recv()
//			if err == io.EOF {
//			break
//		}
//			require.NoError(t, err)
//			lines = append(lines, msg.Content)
//		}
//
//		require.Contains(t, lines, "test value")
//	})
//
//	// Test case 3: Proceso largo y cancelación
//	t.Run("Long Running Process and Cancellation", func(t *testing.T) {
//		ctxWithCancel, cancel := context.WithCancel(ctx)
//		defer cancel()
//
//		req := &mgr.ExecuteCommandRequest{
//			ProcessId:  "test-long-process",
//			Command:    []string{"sh", "-c", "for i in $(seq 1 10); do echo $i; sleep 1; done"},
//			WorkingDir: "/tmp",
//		}
//
//		stream, err := managerClient.ExecuteDistributedCommand(ctxWithCancel, req)
//		require.NoError(t, err)
//
//		var lines []string
//		lineCount := 0
//		for {
//			msg, err := stream.Recv()
//			if err == io.EOF {
//				break
//			}
//			if err != nil {
//				if strings.Contains(err.Error(), "context canceled") {
//					break
//				}
//				require.NoError(t, err)
//			}
//		lines = append(lines, msg.Content)
//			lineCount++
//			if lineCount >= 3 {
//				cancel()
//			}
//		}
//
//		require.Less(t, len(lines), 10, "Process should have been cancelled before completion")
//	})
//
//	// Test case 4: Manejo de errores
//	t.Run("Error Handling", func(t *testing.T) {
//		req := &mgr.ExecuteCommandRequest{
//			ProcessId:  "test-error-cmd",
//			Command:    []string{"sh", "-c", "this_command_does_not_exist"},
//			WorkingDir: "/tmp",
//		}
//
//		stream, err := managerClient.ExecuteDistributedCommand(ctx, req)
//		require.NoError(t, err)
//
//		var errFound bool
//		var lines []string
//		for {
//			msg, err := stream.Recv()
//			if err == io.EOF {
//				break
//			}
//			if err != nil {
//				errFound = true
//				t.Logf("Received error: %v", err)
//				require.Contains(t, err.Error(), "exit status", "Expected error with exit status")
//			break
//			}
//			if msg != nil {
//				t.Logf("Received output: %s", msg.Content)
//				lines = append(lines, msg.Content)
//				if strings.Contains(msg.Content, "command not found") ||
//					strings.Contains(msg.Content, "Worker ocupado") {
//					errFound = true
//				}
//			}
//		}
//
//		if !errFound {
//			t.Logf("All received output: %v", lines)
//		}
//		require.True(t, errFound, "Expected an error, 'command not found', or 'Worker ocupado' message")
//	})
//
//	// Test case 5: Recuperación después de error
//	t.Run("Recovery After Error", func(t *testing.T) {
//		// Esperar un momento para asegurar que el worker esté disponible
//		time.Sleep(5 * time.Second)
//
//		// Ejecutar varios comandos hasta que uno tenga éxito
//		maxAttempts := 5
//		var success bool
//		var lastError error
//
//		for i := 0; i < maxAttempts; i++ {
//			req := &mgr.ExecuteCommandRequest{
//				ProcessId:  fmt.Sprintf("test-recovery-cmd-%d", i),
//				Command:    []string{"sh", "-c", "echo 'recovery test'"},
//				WorkingDir: "/tmp",
//			}
//
//			stream, err := managerClient.ExecuteDistributedCommand(ctx, req)
//			if err != nil {
//				lastError = err
//				time.Sleep(time.Second)
//				continue
//			}
//
//			var lines []string
//			var streamError error
//			for {
//				msg, err := stream.Recv()
//				if err == io.EOF {
//					break
//				}
//				if err != nil {
//					streamError = err
//					break
//				}
//				t.Logf("Received output: %s", msg.Content)
//				lines = append(lines, msg.Content)
//			}
//
//			if streamError != nil {
//				lastError = streamError
//				time.Sleep(time.Second)
//				continue
//			}
//
//			if len(lines) > 0 && lines[0] != "Worker ocupado" {
//				success = true
//				require.Contains(t, lines, "recovery test", "Expected recovery test output")
//				break
//			}
//
//			time.Sleep(time.Second)
//		}
//
//		if !success {
//			t.Fatalf("Failed to execute command after %d attempts. Last error: %v", maxAttempts, lastError)
//		}
//	})
//
//	t.Log("E2E TEST completed successfully.")
//}
