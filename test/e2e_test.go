package test

import (
	"context"
	mgr "dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/manager"
	"fmt"
	"log"
	"strings"

	"testing"

	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	// Cliente gRPC para Manager

	// Aquí asumes que generaste stubs y están disponibles
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestE2E utiliza testcontainers para arrancar contenedores de Manager y Worker.
func TestE2E(t *testing.T) {
	ctx := context.Background()

	// 1. Crear red interna para que manager <-> worker se resuelvan por nombre
	networkName := "devops-platform-testnet"
	networkRequest := testcontainers.NetworkRequest{
		Name:   networkName,
		Driver: "bridge",
	}
	testNet, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: networkRequest,
	})
	require.NoError(t, err, "No se pudo crear la red de test")

	defer func() {
		if testNet != nil {
			_ = testNet.Remove(ctx)
		}
	}()

	// 2. Levantar contenedor del Manager
	managerReq := testcontainers.ContainerRequest{
		Name:         "manager-test",
		Image:        "dev.rubentxu.devops-platform/manager:latest", // Cambiar por la imagen real
		ExposedPorts: []string{"50051/tcp"},
		Networks:     []string{networkName},
		// Esperamos a que en los logs aparezca un mensaje, p.ej. "Worker Manager escuchando en :50051..."
		WaitingFor: wait.ForLog("Worker Manager escuchando"),
	}
	managerC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: managerReq,
		Started:          true,
	})
	require.NoError(t, err, "Fallo al iniciar el contenedor del Manager")

	defer func() {
		if managerC != nil {
			_ = managerC.Terminate(ctx)
		}
	}()

	// 3. Levantar contenedor del Worker
	//    Este worker, al arrancar, se conectará a "manager-test:50051"
	workerReq := testcontainers.ContainerRequest{
		Name:         "worker-test",
		Image:        "dev.rubentxu.devops-platform/worker:latest", // Cambiar por la imagen real
		ExposedPorts: []string{"50052/tcp"},
		Networks:     []string{networkName},
		Env: map[string]string{
			"MANAGER_HOST": "manager-test", // Se resuelve por la red
			"MANAGER_PORT": "50051",
		},
		WaitingFor: wait.ForLog("Worker escuchando"),
	}
	workerC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: workerReq,
		Started:          true,
	})
	require.NoError(t, err, "Fallo al iniciar el contenedor del Worker")

	defer func() {
		if workerC != nil {
			_ = workerC.Terminate(ctx)
		}
	}()

	// 4. Obtener host y puerto del Manager para conectarnos por gRPC
	managerHost, err := managerC.Host(ctx)
	require.NoError(t, err)

	managerPort, err := managerC.MappedPort(ctx, "50051")
	require.NoError(t, err)

	managerAddr := fmt.Sprintf("%s:%d", managerHost, managerPort.Int())
	log.Printf("Manager accesible en %s", managerAddr)

	// 5. Conectarnos al Manager vía gRPC
	conn, err := grpc.Dial(managerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	managerClient := mgr.NewProcessManagementServiceClient(conn)

	// 6. Ejecutar un comando que genere múltiples trazas y tarde un poco
	// Por ejemplo, un script largo con "for i in 1..50"
	// Para la demo, asumimos la API manager -> ExecuteDistributedCommand
	processID := "test-process-123"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "for i in {1..5}; do echo 'Simulando salida'; sleep 1; done; echo 'Fin proceso'; exit 0"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	stream, err := managerClient.ExecuteDistributedCommand(ctx, req)
	require.NoError(t, err)

	// 7. Leer logs por streaming y verificar que obtenemos las salidas esperadas
	var lines []string
	for {
		msg, err := stream.Recv()
		if err != nil {
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			require.NoError(t, err, "Error inesperado recibiendo logs")
		}
		if msg == nil {
			break
		}
		lines = append(lines, msg.Content)
		log.Printf("[LOG stream] %s", msg.Content)

		// Verificar estado final
		if msg.State == "COMPLETED" || msg.State == "FAILED" {
			break
		}
	}

	require.Contains(t, lines, "Simulando salida", "No se recibieron los logs esperados")
	require.Contains(t, lines, "Fin proceso")

	// 8. Test de HealthCheck y Metricas
	//    - Normalmente, Manager usaría streaming para obtener Health & Metrics.
	//    - Aquí, haremos una llamada gRPC extra (si estuviera expuesta) o validamos logs.

	//    - O, si Worker exporta un endpoint "ReportMetrics" en streaming, podríamos conectarnos
	//      del test a la misma interfaz y verificar. Ejemplo (pseudo-código):
	/*
	   workerConn, err := grpc.Dial(workerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	   workerClient := w.NewWorkerProcessServiceClient(workerConn)

	   metricsStream, err := workerClient.ReportMetrics(ctx)
	   // Recibir algunas métricas
	   ...
	*/

	// Para demostrar la lógica, simulamos un assert en logs:
	// (Asumiendo que en logs Worker imprimiría: "CPU usage: 5% ...")
	// require.Contains(t, allWorkerLogs, "CPU usage: 5%")

	// 9. Terminar (si quisiéramos probar la terminación de un proceso en medio de su ejecución)
	//    EJEMPLO: managerClient.TerminateProcess(ctx, &mgr.TerminateProcessRequest{ ProcessId: processID })

	// Fin: el test verifica que todo se completó sin errores
	t.Log("TEST E2E finalizado con éxito.")
}
