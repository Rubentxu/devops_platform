package e2e

import (
	"context"
	mgr "dev.rubentxu/devops_platform/adapters/grpc/protos/manager"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type PerformanceTestSuite struct {
	E2ETestSuite
}

func TestPerformanceSuite(t *testing.T) {
	suite.Run(t, new(PerformanceTestSuite))
}

// Test de ejecución concurrente de comandos
func (s *PerformanceTestSuite) TestConcurrentCommandExecution() {
	numCommands := 100
	var wg sync.WaitGroup
	startTime := time.Now()
	completedCommands := 0
	errorCount := 0

	for i := 0; i < numCommands; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			processID := fmt.Sprintf("concurrent-cmd-%d", i)
			req := &mgr.ExecuteCommandRequest{
				ProcessId:  processID,
				Command:    []string{"/bin/bash", "-c", fmt.Sprintf("echo 'Concurrent command %d'", i)},
				WorkingDir: "/tmp",
				EnvVars:    map[string]string{},
			}

			stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
			if err != nil {
				log.Printf("Error iniciando comando %d: %v", i, err)
				errorCount++
				return
			}

			for {
				msg, err := stream.Recv()
				if err != nil {
					if !strings.Contains(err.Error(), "EOF") {
						log.Printf("Error en stream del comando %d: %v", i, err)
						errorCount++
					}
					break
				}
				if msg == nil {
					break
				}
				if msg.State == "COMPLETED" {
					completedCommands++
					break
				}
				if msg.State == "FAILED" {
					errorCount++
					break
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	s.T().Logf("Tiempo total para %d comandos concurrentes: %v", numCommands, duration)
	s.T().Logf("Promedio por comando: %v", duration/time.Duration(numCommands))
	s.T().Logf("Comandos completados: %d", completedCommands)
	s.T().Logf("Errores encontrados: %d", errorCount)

	s.Greater(completedCommands, numCommands*90/100, "Menos del 90% de los comandos se completaron con éxito")
	s.Less(errorCount, numCommands*10/100, "Más del 10% de los comandos fallaron")
}

// Test de latencia de comandos
func (s *PerformanceTestSuite) TestCommandLatency() {
	numSamples := 50
	latencies := make([]time.Duration, 0, numSamples)

	for i := 0; i < numSamples; i++ {
		start := time.Now()
		processID := fmt.Sprintf("latency-test-%d", i)
		req := &mgr.ExecuteCommandRequest{
			ProcessId:  processID,
			Command:    []string{"/bin/bash", "-c", "echo 'test'"},
			WorkingDir: "/tmp",
			EnvVars:    map[string]string{},
		}

		stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
		s.Require().NoError(err)

		for {
			msg, err := stream.Recv()
			if err != nil {
				if !strings.Contains(err.Error(), "EOF") {
					s.Require().NoError(err)
				}
				break
			}
			if msg == nil {
				break
			}
			if msg.State == "COMPLETED" || msg.State == "FAILED" {
				break
			}
		}

		latencies = append(latencies, time.Since(start))
	}

	// Calcular estadísticas
	var total time.Duration
	var max time.Duration
	min := latencies[0]

	for _, lat := range latencies {
		total += lat
		if lat > max {
			max = lat
		}
		if lat < min {
			min = lat
		}
	}

	avg := total / time.Duration(len(latencies))

	s.T().Logf("Latencia promedio: %v", avg)
	s.T().Logf("Latencia máxima: %v", max)
	s.T().Logf("Latencia mínima: %v", min)

	s.Less(avg, 500*time.Millisecond, "La latencia promedio es mayor a 500ms")
	s.Less(max, time.Second, "La latencia máxima es mayor a 1s")
}

// Test de carga sostenida
func (s *PerformanceTestSuite) TestSustainedLoad() {
	duration := 1 * time.Minute
	interval := 100 * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	ctx, cancel := context.WithTimeout(s.ctx, duration)
	defer cancel()

	var completedCommands int
	var errorCount int
	var wg sync.WaitGroup

	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			s.T().Logf("Comandos completados en %v: %d", duration, completedCommands)
			s.T().Logf("Errores encontrados: %d", errorCount)
			s.Less(float64(errorCount)/float64(completedCommands), 0.01, "Tasa de error mayor al 1%")
			return
		case <-ticker.C:
			wg.Add(1)
			go func() {
				defer wg.Done()
				processID := fmt.Sprintf("sustained-load-%d", time.Now().UnixNano())
				req := &mgr.ExecuteCommandRequest{
					ProcessId:  processID,
					Command:    []string{"/bin/bash", "-c", "echo 'sustained load test'"},
					WorkingDir: "/tmp",
					EnvVars:    map[string]string{},
				}

				stream, err := s.managerClient.ExecuteDistributedCommand(ctx, req)
				if err != nil {
					errorCount++
					return
				}

				for {
					msg, err := stream.Recv()
					if err != nil {
						if !strings.Contains(err.Error(), "EOF") {
							errorCount++
						}
						break
					}
					if msg == nil {
						break
					}
					if msg.State == "COMPLETED" {
						completedCommands++
						break
					}
					if msg.State == "FAILED" {
						errorCount++
						break
					}
				}
			}()
		}
	}
}

// Test de recuperación bajo carga
func (s *PerformanceTestSuite) TestRecoveryUnderLoad() {
	// Iniciar carga de fondo
	ctx, cancel := context.WithCancel(s.ctx)
	var wg sync.WaitGroup
	completedBefore := 0
	errorsBefore := 0
	completedAfter := 0
	errorsAfter := 0
	recoveryStarted := make(chan struct{})

	// Iniciar workers de carga
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
					processID := fmt.Sprintf("recovery-load-%d-%d", workerID, time.Now().UnixNano())
					req := &mgr.ExecuteCommandRequest{
						ProcessId:  processID,
						Command:    []string{"/bin/bash", "-c", "echo 'background load'"},
						WorkingDir: "/tmp",
						EnvVars:    map[string]string{},
					}

					stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
					if err != nil {
						select {
						case <-recoveryStarted:
							errorsAfter++
						default:
							errorsBefore++
						}
						continue
					}

					success := false
					for {
						msg, err := stream.Recv()
						if err != nil {
							if !strings.Contains(err.Error(), "EOF") {
								select {
								case <-recoveryStarted:
									errorsAfter++
								default:
									errorsBefore++
								}
							}
							break
						}
						if msg == nil {
							break
						}
						if msg.State == "COMPLETED" {
							success = true
							break
						}
						if msg.State == "FAILED" {
							select {
							case <-recoveryStarted:
								errorsAfter++
							default:
								errorsBefore++
							}
							break
						}
					}

					if success {
						select {
						case <-recoveryStarted:
							completedAfter++
						default:
							completedBefore++
						}
					}

					time.Sleep(100 * time.Millisecond)
				}
			}
		}(i)
	}

	// Esperar a que se establezca la carga
	time.Sleep(5 * time.Second)

	// Matar y reiniciar worker1
	err := s.worker1Cont.Stop(s.ctx, nil)
	s.Require().NoError(err)
	close(recoveryStarted)
	time.Sleep(2 * time.Second)

	err = s.worker1Cont.Start(s.ctx)
	s.Require().NoError(err)
	time.Sleep(5 * time.Second)

	// Detener la carga y analizar resultados
	cancel()
	wg.Wait()

	s.T().Logf("Comandos completados antes de la falla: %d", completedBefore)
	s.T().Logf("Errores antes de la falla: %d", errorsBefore)
	s.T().Logf("Comandos completados después de la falla: %d", completedAfter)
	s.T().Logf("Errores después de la falla: %d", errorsAfter)

	s.Greater(completedAfter, 0, "No se completaron comandos después de la recuperación")
	s.Less(float64(errorsAfter)/float64(completedAfter), 0.2, "Tasa de error post-recuperación mayor al 20%")
}

// Test de límites del sistema
func (s *PerformanceTestSuite) TestSystemLimits() {
	// Test de comando con salida grande
	processID := "large-output-test"
	req := &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "dd if=/dev/zero bs=1M count=100 | base64"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	start := time.Now()
	stream, err := s.managerClient.ExecuteDistributedCommand(s.ctx, req)
	s.Require().NoError(err)

	var outputSize int64
	for {
		msg, err := stream.Recv()
		if err != nil {
			if !strings.Contains(err.Error(), "EOF") {
				s.Require().NoError(err)
			}
			break
		}
		if msg == nil {
			break
		}
		outputSize += int64(len(msg.Content))
		if msg.State == "COMPLETED" || msg.State == "FAILED" {
			break
		}
	}

	duration := time.Since(start)
	throughput := float64(outputSize) / duration.Seconds() / 1024 / 1024 // MB/s

	s.T().Logf("Throughput para salida grande: %.2f MB/s", throughput)
	s.Greater(throughput, 1.0, "Throughput menor a 1 MB/s")

	// Test de comando con uso intensivo de CPU
	processID = "cpu-intensive-test"
	req = &mgr.ExecuteCommandRequest{
		ProcessId:  processID,
		Command:    []string{"/bin/bash", "-c", "openssl speed sha256"},
		WorkingDir: "/tmp",
		EnvVars:    map[string]string{},
	}

	start = time.Now()
	stream, err = s.managerClient.ExecuteDistributedCommand(s.ctx, req)
	s.Require().NoError(err)

	for {
		msg, err := stream.Recv()
		if err != nil {
			if !strings.Contains(err.Error(), "EOF") {
				s.Require().NoError(err)
			}
			break
		}
		if msg == nil {
			break
		}
		if msg.State == "COMPLETED" || msg.State == "FAILED" {
			break
		}
	}

	duration = time.Since(start)
	s.T().Logf("Tiempo para comando CPU-intensivo: %v", duration)
	s.Less(duration, 30*time.Second, "El comando CPU-intensivo tardó más de 30 segundos")
}
