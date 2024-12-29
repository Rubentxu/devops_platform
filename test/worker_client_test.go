package test

import (
	"bufio"
	"context"
	"fmt"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"dev.rubentxu.devops-platform/adapters/grpc/grpc/client"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/worker"
	"dev.rubentxu.devops-platform/adapters/grpc/logger"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WorkerClientTestSuite struct {
	suite.Suite
	ctx          context.Context
	projectName  string
	composeFile  string
	workerClient *client.WorkerClient
}

func TestWorkerClientSuite(t *testing.T) {
	suite.Run(t, new(WorkerClientTestSuite))
}

func (s *WorkerClientTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.projectName = fmt.Sprintf("worker-client-test-%d", time.Now().UnixNano())

	// Usar testcontainers-go para manejar Docker Compose
	compose, err := tc.NewDockerCompose("docker-compose.yaml")
	s.Require().NoError(err, "NewDockerCompose()")

	err = compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal)
	if err != nil {
		s.T().Logf("Warning during initial cleanup: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	s.T().Cleanup(cancel)

	// Mostrar logs de los contenedores mientras se inician los servicios
	showContainerLogs := func(service string) {
		container, err := compose.ServiceContainer(ctx, service)
		if err != nil {
			s.T().Logf("Error getting %s container: %v", service, err)
			return
		}
		logs, err := container.Logs(ctx)
		if err != nil {
			s.T().Logf("Error getting %s logs: %v", service, err)
			return
		}
		defer logs.Close()
		scanner := bufio.NewScanner(logs)
		for scanner.Scan() {
			s.T().Logf("%s: %s", service, scanner.Text())
		}
	}

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
	s.T().Log("Starting services...")
	err = compose.Up(ctx)
	s.Require().NoError(err, "compose.Up()")
	s.T().Log("Services started successfully")

	// Dar tiempo inicial para que los servicios se inicien
	time.Sleep(5 * time.Second)

	// Crear cliente gRPC conectando al host local
	clientAddr := "localhost:50052"
	conn, err := waitForClientGRPCService(clientAddr)
	s.Require().NoError(err)

	zapLogger, err := logger.NewZapLogger()
	s.Require().NoError(err)
	s.workerClient = client.NewWorkerClient(conn, zapLogger)
}

func waitForClientGRPCService(addr string) (*grpc.ClientConn, error) {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithBlock(),
			grpc.WithTimeout(2*time.Second))
		if err == nil {
			conn.Close()
			return nil, err
		}
		return conn, nil
	}
	return nil, fmt.Errorf("service at %s not available after timeout", addr)
}

func (s *WorkerClientTestSuite) TearDownSuite() {
	// Ejecutar docker compose down
	err := RunCommand("docker", "compose", "-f", s.composeFile, "-p", s.projectName, "down", "-v")
	s.Require().NoError(err)

	// Limpiar archivos temporales
	os.RemoveAll(filepath.Dir(s.composeFile))
}

// RunCommand ejecuta un comando y retorna error si falla
func RunCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *WorkerClientTestSuite) TestExecuteCommand() {
	// Canal para recibir la salida del comando
	outputChan := make(chan *client.CommandOutput)

	// Ejecutar comando
	err := s.workerClient.ExecuteCommand(s.ctx, "test-1", []string{"echo", "hello world"}, "/tmp", nil, outputChan)
	s.Require().NoError(err)

	// Recolectar salida
	var outputs []*client.CommandOutput
	for output := range outputChan {
		outputs = append(outputs, output)
	}

	// Verificar resultados
	s.Require().NotEmpty(outputs)
	s.Equal("hello world", outputs[0].Content)
	s.Equal("completed", outputs[len(outputs)-1].State)
	s.Equal(int32(0), outputs[len(outputs)-1].ExitCode)
}

func (s *WorkerClientTestSuite) TestExecuteCommandError() {
	outputChan := make(chan *client.CommandOutput)

	// Ejecutar comando que fallará
	err := s.workerClient.ExecuteCommand(s.ctx, "test-2", []string{"nonexistentcommand"}, "/tmp", nil, outputChan)
	s.Require().NoError(err)

	// Recolectar salida
	var outputs []*client.CommandOutput
	for output := range outputChan {
		outputs = append(outputs, output)
	}

	// Verificar que el comando falló
	s.Require().NotEmpty(outputs)
	s.Equal("failed", outputs[len(outputs)-1].State)
	s.NotEqual(int32(0), outputs[len(outputs)-1].ExitCode)
}

func (s *WorkerClientTestSuite) TestHealthCheck() {
	healthChan := make(chan *worker.HealthCheckResponse)

	// Iniciar health check
	err := s.workerClient.StartHealthCheck(s.ctx, "worker1", healthChan, 1*time.Second)
	s.Require().NoError(err)

	// Esperar primera respuesta
	select {
	case resp := <-healthChan:
		s.True(resp.IsHealthy)
	case <-time.After(2 * time.Second):
		s.Fail("timeout waiting for health check response")
	}
}

func (s *WorkerClientTestSuite) TestMetricsCollection() {
	metricsChan := make(chan *worker.MetricsResponse)

	// Iniciar recolección de métricas
	err := s.workerClient.StartMetricsCollection(s.ctx, "worker1", metricsChan, 1*time.Second)
	s.Require().NoError(err)

	// Esperar primera métrica
	select {
	case metrics := <-metricsChan:
		s.NotEmpty(metrics.CpuUsage)
		s.NotEmpty(metrics.MemUsage)
	case <-time.After(2 * time.Second):
		s.Fail("timeout waiting for metrics")
	}
}
