package e2e

import (
	"context"
	mgr "dev.rubentxu/devops_platform/adapters/grpc/protos/manager"
	"fmt"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type E2ETestSuite struct {
	suite.Suite
	ctx            context.Context
	managerCont    testcontainers.Container
	worker1Cont    testcontainers.Container
	worker2Cont    testcontainers.Container
	managerClient  mgr.ProcessManagementServiceClient
	managerConn    *grpc.ClientConn
	managerAddress string
	network        testcontainers.Network
}

func TestE2ESuite(t *testing.T) {
	suite.Run(t, new(E2ETestSuite))
}

func (s *E2ETestSuite) SetupSuite() {
	s.ctx = context.Background()

	// Crear una red única para cada test
	networkName := fmt.Sprintf("devops-platform-testnet-%d", time.Now().UnixNano())
	networkRequest := testcontainers.NetworkRequest{
		Name:   networkName,
		Driver: "bridge",
	}
	testNet, err := testcontainers.GenericNetwork(s.ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: networkRequest,
	})
	s.Require().NoError(err, "No se pudo crear la red de test")
	s.network = testNet

	// 2. Levantar contenedor del Manager
	managerReq := testcontainers.ContainerRequest{
		Name:         fmt.Sprintf("manager-test-%d", time.Now().UnixNano()),
		Image:        "devops-platform/manager:latest",
		ExposedPorts: []string{"50051/tcp"},
		Networks:     []string{networkName},
		WaitingFor:   wait.ForLog("Worker Manager escuchando"),
	}
	managerC, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: managerReq,
		Started:          true,
	})
	s.Require().NoError(err, "Fallo al iniciar el contenedor del Manager")
	s.managerCont = managerC

	// 3. Levantar contenedores de Worker
	workerReq := testcontainers.ContainerRequest{
		Name:         fmt.Sprintf("worker-test-1-%d", time.Now().UnixNano()),
		Image:        "devops-platform/worker:latest",
		ExposedPorts: []string{"50052/tcp"},
		Networks:     []string{networkName},
		Env: map[string]string{
			"MANAGER_HOST": managerReq.Name,
			"MANAGER_PORT": "50051",
		},
		WaitingFor: wait.ForLog("Worker escuchando"),
	}

	// Worker 1
	worker1C, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: workerReq,
		Started:          true,
	})
	s.Require().NoError(err, "Fallo al iniciar el contenedor del Worker 1")
	s.worker1Cont = worker1C

	// Worker 2
	workerReq.Name = fmt.Sprintf("worker-test-2-%d", time.Now().UnixNano())
	workerReq.Env["MANAGER_HOST"] = managerReq.Name
	worker2C, err := testcontainers.GenericContainer(s.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: workerReq,
		Started:          true,
	})
	s.Require().NoError(err, "Fallo al iniciar el contenedor del Worker 2")
	s.worker2Cont = worker2C

	// 4. Obtener host y puerto del Manager para conectarnos por gRPC
	managerHost, err := managerC.Host(s.ctx)
	s.Require().NoError(err)

	managerPort, err := managerC.MappedPort(s.ctx, "50051")
	s.Require().NoError(err)

	s.managerAddress = fmt.Sprintf("%s:%d", managerHost, managerPort.Int())
	log.Printf("Manager accesible en %s", s.managerAddress)

	// 5. Conectarnos al Manager vía gRPC
	conn, err := grpc.Dial(s.managerAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	s.Require().NoError(err)
	s.managerConn = conn
	s.managerClient = mgr.NewProcessManagementServiceClient(conn)

	// 6. Esperar a que los workers se registren
	time.Sleep(10 * time.Second)
}

func (s *E2ETestSuite) TearDownSuite() {
	if s.managerConn != nil {
		s.managerConn.Close()
	}
	if s.managerCont != nil {
		s.managerCont.Terminate(s.ctx)
	}
	if s.worker1Cont != nil {
		s.worker1Cont.Terminate(s.ctx)
	}
	if s.worker2Cont != nil {
		s.worker2Cont.Terminate(s.ctx)
	}
	if s.network != nil {
		s.network.Remove(s.ctx)
	}
}

// Test de ejecución básica de comando
func (s *E2ETestSuite) TestBasicCommandExecution() {
	processID := "test-basic-cmd"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "echo 'Hello World'"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
	s.Require().NoError(err)

	var lines []string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			s.Require().NoError(err, "Error inesperado recibiendo logs")
		}
		if msg == nil {
			break
		}
		lines = append(lines, msg.Content)
		log.Printf("[LOG stream] %s", msg.Content)

		if msg.State == "COMPLETED" || msg.State == "FAILED" {
			break
		}
	}

	s.Contains(lines, "Hello World", "No se recibió la salida esperada")
}

// Test de proceso largo y cancelación
func (s *E2ETestSuite) TestLongRunningProcessAndCancellation() {
	processID := "test-long-process"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "for i in {1..30}; do echo $i; sleep 1; done"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
	s.Require().NoError(err)

	// Esperar a que el proceso empiece
	time.Sleep(2 * time.Second)

	// Intentar cancelar el proceso
	terminateReq := &mgr.TerminateProcessRequest{
		ProcessId: processID,
	}
	_, err = s.managerClient.TerminateProcess(s.ctx, terminateReq)
	s.Require().NoError(err)

	// Verificar que el proceso fue terminado
	var lastState string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			s.Require().NoError(err)
		}
		if msg == nil {
			break
		}
		lastState = msg.State
		if msg.State == "ABORTED" || msg.State == "FAILED" {
			break
		}
	}

	s.Equal("ABORTED", lastState, "El proceso no fue abortado correctamente")
}

// Test de proceso zombie
func (s *E2ETestSuite) TestZombieProcessHandling() {
	processID := "test-zombie-process"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "sleep 100 & disown"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
	s.Require().NoError(err)

	// Esperar a que el proceso empiece
	time.Sleep(2 * time.Second)

	// Matar el worker1 para simular un fallo
	err = s.worker1Cont.Stop(s.ctx, nil)
	s.Require().NoError(err)

	// Verificar que el proceso es marcado como fallido
	var lastState string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			s.Require().NoError(err)
		}
		if msg == nil {
			break
		}
		lastState = msg.State
		if msg.State == "FAILED" {
			break
		}
	}

	s.Equal("FAILED", lastState, "El proceso zombie no fue detectado correctamente")
}

// Test de error handling
func (s *E2ETestSuite) TestErrorHandling() {
	processID := "test-error-handling"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "invalid_command"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
	s.Require().NoError(err)

	var lastState string
	var errorMsg string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			s.Require().NoError(err)
		}
		if msg == nil {
			break
		}
		lastState = msg.State
		errorMsg = msg.ErrorMsg
		if msg.State == "FAILED" {
			break
		}
	}

	s.Equal("FAILED", lastState)
	s.Contains(errorMsg, "command not found")
}

// Test de recuperación tras fallo
func (s *E2ETestSuite) TestFailureRecovery() {
	// Detener worker1
	err := s.worker1Cont.Stop(s.ctx, nil)
	s.Require().NoError(err)

	time.Sleep(2 * time.Second)

	// Reiniciar worker1
	err = s.worker1Cont.Start(s.ctx)
	s.Require().NoError(err)

	time.Sleep(5 * time.Second)

	// Intentar ejecutar un comando
	processID := "test-recovery"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "echo 'recovered'"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
	s.Require().NoError(err)

	var lines []string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			s.Require().NoError(err)
		}
		if msg == nil {
			break
		}
		lines = append(lines, msg.Content)
		if msg.State == "COMPLETED" || msg.State == "FAILED" {
			break
		}
	}

	s.Contains(lines, "recovered", "El sistema no se recuperó correctamente")
}
