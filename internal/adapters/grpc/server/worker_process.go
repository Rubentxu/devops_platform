package server

import (
	"bufio"
	"context"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/worker"

	"io"
	"log"
	"os/exec"
	"strings"
	"time"
)

// WorkerProcessService implementa el servicio WorkerProcessService para ejecutar procesos.
type WorkerProcessService struct {
	worker.UnimplementedWorkerProcessServiceServer
	busy bool
}

// ExecuteCommand usa streaming bidireccional:
// - El client (WorkerManager) podría enviar parámetros en varios mensajes (no siempre es necesario).
// - El worker devuelve la salida en streaming.
func (ws *WorkerProcessService) ExecuteCommand(stream worker.WorkerProcessService_ExecuteCommandServer) error {
	if ws.busy {
		// Si el worker ya está ocupado
		return stream.SendMsg(&worker.CommandOutput{
			ProcessId: "0",
			IsStderr:  true,
			Content:   "Worker ocupado",
			State:     "BUSY",
			ExitCode:  0,
			ErrorMsg:  "",
		})
	}
	ws.busy = true
	defer func() { ws.busy = false }()

	var processID string
	var cmd *exec.Cmd

	// 1. Recibir primer mensaje con la configuración principal.
	req, err := stream.Recv()
	if err != nil {
		return err
	}
	processID = req.ProcessId
	if len(req.Command) == 0 {
		return stream.SendMsg(&worker.CommandOutput{
			ProcessId: processID,
			IsStderr:  true,
			Content:   "No se especificó comando",
			State:     "FAILED",
			ExitCode:  1,
			ErrorMsg:  "",
		})
	}

	cmd = exec.Command(req.Command[0], req.Command[1:]...)
	cmd.Dir = req.WorkingDir
	// Setear variables de entorno
	var env []string
	for k, v := range req.EnvVars {
		env = append(env, k+"="+v)
	}
	cmd.Env = append(cmd.Env, env...)

	// 2. Capturar stdout y stderr
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	// 3. Iniciar el proceso
	if err := cmd.Start(); err != nil {
		_ = stream.Send(&worker.CommandOutput{
			ProcessId: processID,
			IsStderr:  true,
			Content:   "Error al iniciar proceso: " + err.Error(),
			State:     "FAILED",
		})
		return err
	}

	// Goroutine para leer stdout
	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Error leyendo stdout: %v", err)
				break
			}
			_ = stream.Send(&worker.CommandOutput{
				ProcessId: processID,
				IsStderr:  false,
				Content:   strings.TrimSpace(line),
				State:     "RUNNING",
			})
		}
	}()

	// Goroutine para leer stderr
	go func() {
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil && err == io.EOF {
				break
			}
			if err != nil {
				log.Printf("Error leyendo stderr: %v", err)
				break
			}
			_ = stream.Send(&worker.CommandOutput{
				ProcessId: processID,
				IsStderr:  true,
				Content:   strings.TrimSpace(line),
				State:     "RUNNING",
			})
		}
	}()

	// 4. Esperar a que termine el proceso
	err = cmd.Wait()
	exitCode := int32(0)
	state := "COMPLETED"
	errMsg := ""

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = int32(exitError.ExitCode())
		}
		state = "FAILED"
		errMsg = err.Error()
	}

	// 5. Enviar mensaje final con el resultado
	return stream.Send(&worker.CommandOutput{
		ProcessId: processID,
		IsStderr:  false,
		Content:   "Proceso finalizado",
		State:     state,
		ExitCode:  exitCode,
		ErrorMsg:  errMsg,
	})
}

func (ws *WorkerProcessService) TerminateProcess(ctx context.Context, req *worker.TerminateProcessRequest) (*worker.TerminateProcessResponse, error) {
	// En un caso real deberíamos enviar una señal al proceso en ejecución
	// o manejar un mapeo processID->cmd para poder matarlo.
	// Aquí sólo es un ejemplo.
	return &worker.TerminateProcessResponse{
		Success: true,
		Message: "Proceso terminado (simulado)",
	}, nil
}

func (ws *WorkerProcessService) HealthCheck(stream worker.WorkerProcessService_HealthCheckServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// Enviar respuesta
		resp := worker.HealthCheckResponse{
			IsHealthy: true,
			Status:    "Worker OK",
		}
		if err := stream.Send(&resp); err != nil {
			return err
		}
		time.Sleep(5 * time.Second) // Simulamos envío periódico
	}
}

func (ws *WorkerProcessService) ReportMetrics(stream worker.WorkerProcessService_ReportMetricsServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// Leer métricas de /proc, simplificado
		cpuUsage := "5%"    // Ejemplo
		memUsage := "128MB" // Ejemplo
		diskUsage := "10GB" // Ejemplo

		resp := worker.MetricsResponse{
			CpuUsage:  cpuUsage,
			MemUsage:  memUsage,
			DiskUsage: diskUsage,
		}
		if err := stream.Send(&resp); err != nil {
			return err
		}
		time.Sleep(5 * time.Second)
	}
}
