package client

import (
	"context"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/worker"
	"dev.rubentxu.devops-platform/core/ports"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// WorkerClient encapsula la funcionalidad del cliente gRPC para el Worker
type WorkerClient struct {
	client worker.WorkerProcessServiceClient
	logger ports.Logger
}

// NewWorkerClient crea una nueva instancia del cliente
func NewWorkerClient(conn *grpc.ClientConn, logger ports.Logger) *WorkerClient {
	return &WorkerClient{
		client: worker.NewWorkerProcessServiceClient(conn),
		logger: logger.With("component", "worker_client"),
	}
}

// ExecuteCommand ejecuta un comando en el worker
func (c *WorkerClient) ExecuteCommand(
	ctx context.Context,
	processID string,
	command []string,
	workingDir string,
	envVars map[string]string,
	outputChan chan<- *ports.CommandOutput) error {

	workerStream, err := c.client.ExecuteCommand(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create command workerStream")
	}

	// Enviar la solicitud
	req := &worker.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    command,
		WorkingDir: workingDir,
		EnvVars:    envVars,
	}

	if err := workerStream.Send(req); err != nil {
		return errors.Wrap(err, "failed to send command request to worker")
	}

	// Procesar respuestas en una goroutine
	go func() {
		defer close(outputChan)
		for {
			resp, err := workerStream.Recv()
			if err == io.EOF {
				return
			}
			if err != nil {
				c.logger.Error("error receiving command output", "error", err)
				outputChan <- &ports.CommandOutput{
					ProcessID: processID,
					State:     "failed",
					ErrorMsg:  err.Error(),
				}
				return
			}

			output := &ports.CommandOutput{
				ProcessID: resp.ProcessId,
				IsStderr:  resp.IsStderr,
				Content:   resp.Content,
				State:     resp.State,
				ExitCode:  resp.ExitCode,
				ErrorMsg:  resp.ErrorMsg,
			}

			select {
			case outputChan <- output:
			case <-ctx.Done():
				return
			}

			if resp.State == "completed" || resp.State == "failed" {
				return
			}
		}
	}()

	return nil
}

// TerminateProcess termina un proceso en ejecución
func (c *WorkerClient) TerminateProcess(ctx context.Context, processID string) error {
	resp, err := c.client.TerminateProcess(ctx, &worker.TerminateProcessRequest{
		ProcessId: processID,
	})
	if err != nil {
		return errors.Wrap(err, "failed to terminate process")
	}

	if !resp.Success {
		return fmt.Errorf("failed to terminate process: %s", resp.Message)
	}

	return nil
}

// StartHealthCheck inicia el monitoreo de salud del worker
func (c *WorkerClient) StartHealthCheck(ctx context.Context, workerID string, healthChan chan<- *worker.HealthCheckResponse, interval time.Duration) error {
	stream, err := c.client.HealthCheck(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create health check stream")
	}

	// Procesar respuestas en una goroutine
	go func() {
		defer close(healthChan)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Enviar solicitud de health check
				if err := stream.Send(&worker.HealthCheckRequest{
					WorkerId: workerID,
				}); err != nil {
					c.logger.Error("error sending health check request", "error", err)
					return
				}

				// Recibir respuesta
				resp, err := stream.Recv()
				if err == io.EOF {
					return
				}
				if err != nil {
					c.logger.Error("error receiving health check", "error", err)
					return
				}

				select {
				case healthChan <- resp:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// StartMetricsCollection inicia la recolección de métricas del worker
func (c *WorkerClient) StartMetricsCollection(ctx context.Context, workerID string, metricsChan chan<- *worker.MetricsResponse, interval time.Duration) error {
	stream, err := c.client.ReportMetrics(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create metrics stream")
	}

	// Procesar respuestas en una goroutine
	go func() {
		defer close(metricsChan)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Enviar solicitud de métricas
				if err := stream.Send(&worker.MetricsRequest{
					WorkerId: workerID,
				}); err != nil {
					c.logger.Error("error sending metrics request", "error", err)
					return
				}

				// Recibir respuesta
				resp, err := stream.Recv()
				if err == io.EOF {
					return
				}
				if err != nil {
					c.logger.Error("error receiving metrics", "error", err)
					return
				}

				select {
				case metricsChan <- resp:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}
