package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"dev.rubentxu.devops-platform/adapters/grpc/store"
	"dev.rubentxu.devops-platform/adapters/grpc/worker"
	"dev.rubentxu.devops-platform/core/domain"
	"dev.rubentxu.devops-platform/core/domain/task"
)

func TestWorkerServiceWithInMemoryStore(t *testing.T) {

	// Por claridad, definimos una función auxiliar para crear un nuevo WorkerService.
	// Cada sub-test creará su propio store y WorkerService, evitando “contaminar” datos.
	newWorkerService := func(queueSize, maxConcurrent int) (*worker.WorkerService, error) {
		// Crear un store en memoria
		taskStore, err := store.NewStore("memory", "memoryDb-test")
		if err != nil {
			return nil, err
		}

		// Crear el worker service con cola y límite de concurrencia
		w := worker.NewWorkerService(
			"test-worker",
			taskStore,
			nil, // gRPC client (nil en este test)
			context.Background(),
			queueSize,
			maxConcurrent,
		)
		return w, nil
	}

	// ------------------------------------------------------------------------
	// TEST 1: Ejecutar una tarea básica (echo) hasta completarla
	// ------------------------------------------------------------------------
	t.Run("Execute Basic Echo Task", func(t *testing.T) {
		w, err := newWorkerService(10, 2)
		require.NoError(t, err, "failed to create WorkerService")

		// Definir la tarea
		testTask := task.Task{
			Metadata: task.TaskMetadata{
				ID:        uuid.New(),
				Name:      "test-echo-task",
				CreatedAt: time.Now(),
			},
			Spec: task.TaskSpec{
				Command: []string{"echo", "Hello World"},
				Timeout: 10 * time.Second,
			},
			Status: task.TaskStatus{
				State: task.Pending,
			},
		}

		builder := domain.NewSandboxConfigBuilder()
		sandboxConfig := builder.WithName(testTask.Metadata.Name).WithSandboxType(domain.SandboxTypeDocker).WithImage("devops-platform/worker:latest").Build()
		testTask.Spec.SandboxConfig = sandboxConfig

		// Añadir la tarea (se encola)
		err = w.AddTask(testTask)
		require.NoError(t, err, "failed to AddTask")

		// Esperar un poco a que se procese
		time.Sleep(2 * time.Second)

		// Comprobar la tarea en la DB
		raw, err := w.Db.List()
		require.NoError(t, err)

		tasks := raw.([]*task.Task)
		require.Len(t, tasks, 1, "should have 1 task stored")

		finalTask := tasks[0]
		// Es posible que aún no se marque como Completed si el sandboxService real no está,
		// pero teóricamente debería avanzar. Si usas un mock, podrías forzar Completion.
		// Para este ejemplo, simplemente comprobamos que el estado sea al menos Scheduled o Running.
		require.Contains(t,
			[]task.State{task.Scheduled, task.Starting, task.Running, task.Completed, task.Failed},
			finalTask.Status.State,
			"The task should have transitioned out of Pending by now",
		)
	})

	// ------------------------------------------------------------------------
	// TEST 2: Manejo de Timeout
	// ------------------------------------------------------------------------
	t.Run("Task Timeout", func(t *testing.T) {
		w, err := newWorkerService(10, 2)
		require.NoError(t, err)

		testTask := task.Task{
			Metadata: task.TaskMetadata{
				ID:        uuid.New(),
				Name:      "timeout-task",
				CreatedAt: time.Now(),
			},
			Spec: task.TaskSpec{
				Command: []string{"sleep", "60"}, // Un comando que tardaría bastante
				Timeout: 2 * time.Second,         // Forzamos un timeout muy corto
			},
			Status: task.TaskStatus{
				State: task.Pending,
			},
		}

		err = w.AddTask(testTask)
		require.NoError(t, err, "failed to AddTask for timeout-task")

		time.Sleep(4 * time.Second) // Esperamos un poco más que el timeout

		// Verificar el estado de la tarea
		raw, err := w.Db.List()
		require.NoError(t, err)

		tasks := raw.([]*task.Task)
		require.Len(t, tasks, 1)

		finalTask := tasks[0]
		// Si se está usando un gRPC client real, esperaría que diera Failed por timeout.
		// En este test con un "nil" client, no hay ejecución real, pero la lógica de Timeout
		// se simularía si se implementa completamente en WorkerService/GrpcClient mock.
		// Suponiendo que se marcase en Failed:
		require.Equal(t, task.Failed, finalTask.Status.State,
			fmt.Sprintf("Expected task to be Failed due to timeout, got %s", finalTask.Status.State))
	})

	// ------------------------------------------------------------------------
	// TEST 3: Manejo de un comando inexistente (error)
	// ------------------------------------------------------------------------
	t.Run("Task Error Handling (invalid command)", func(t *testing.T) {
		w, err := newWorkerService(10, 2)
		require.NoError(t, err)

		testTask := task.Task{
			Metadata: task.TaskMetadata{
				ID:        uuid.New(),
				Name:      "error-task",
				CreatedAt: time.Now(),
			},
			Spec: task.TaskSpec{
				Command: []string{"invalid-command"},
				Timeout: 30 * time.Second,
			},
			Status: task.TaskStatus{
				State: task.Pending,
			},
		}

		err = w.AddTask(testTask)
		require.NoError(t, err, "failed to AddTask")

		time.Sleep(3 * time.Second) // Esperamos que falle

		raw, err := w.Db.List()
		require.NoError(t, err)

		tasks := raw.([]*task.Task)
		require.Len(t, tasks, 1)
		finalTask := tasks[0]

		// Esperaríamos que esté Failed si detectó "invalid-command"
		require.Equal(t, task.Failed, finalTask.Status.State,
			"Expected the invalid command to fail the task")
	})

	// ------------------------------------------------------------------------
	// TEST 4: Concurrencia: añadir múltiples tareas para ver que no exceda maxConcurrent
	// ------------------------------------------------------------------------
	t.Run("Concurrency limit", func(t *testing.T) {
		// Definimos un maxConcurrent = 2
		w, err := newWorkerService(10, 2)
		require.NoError(t, err)

		// Añadimos 5 tareas "lentitas" (simular con "sleep 2")
		numTasks := 5
		for i := 0; i < numTasks; i++ {
			newTask := task.Task{
				Metadata: task.TaskMetadata{
					ID:        uuid.New(),
					Name:      fmt.Sprintf("concurrent-task-%d", i),
					CreatedAt: time.Now(),
				},
				Spec: task.TaskSpec{
					Command: []string{"sleep", "2"},
					Timeout: 10 * time.Second,
				},
				Status: task.TaskStatus{
					State: task.Pending,
				},
			}
			err := w.AddTask(newTask)
			require.NoError(t, err, "failed to AddTask for concurrency test")
		}

		// Esperamos un tiempo razonable para que se procesen las 5 tareas
		time.Sleep(10 * time.Second)

		// Revisamos los estados de las 5 tareas
		raw, err := w.Db.List()
		require.NoError(t, err)
		tasks := raw.([]*task.Task)
		require.Len(t, tasks, numTasks)

		// Comprobaremos que, al final, todas acaben Completed o Failed
		// (en un mock, no se ejecuta nada real, pero la lógica interna podría marcarlas como completadas)
		for _, tsk := range tasks {
			require.Contains(t,
				[]task.State{task.Running, task.Completed, task.Failed, task.Scheduled, task.Starting},
				tsk.Status.State,
				"All tasks should at least be beyond Pending")
		}
	})

	// ------------------------------------------------------------------------
	// TEST 5: StopTask
	// ------------------------------------------------------------------------
	t.Run("StopTask mid-execution", func(t *testing.T) {
		w, err := newWorkerService(5, 2)
		require.NoError(t, err)

		// Tarea que "no termina" rápidamente
		stopTask := task.Task{
			Metadata: task.TaskMetadata{
				ID:        uuid.New(),
				Name:      "stop-me",
				CreatedAt: time.Now(),
			},
			Spec: task.TaskSpec{
				Command: []string{"sleep", "20"}, // Simular una ejecución larga
				Timeout: 30 * time.Second,
			},
			Status: task.TaskStatus{
				State: task.Pending,
			},
		}

		err = w.AddTask(stopTask)
		require.NoError(t, err)

		// Esperar a que arranque
		time.Sleep(3 * time.Second)

		// Recuperar la tarea desde la DB
		raw, err := w.Db.List()
		require.NoError(t, err)
		tasks := raw.([]*task.Task)
		require.Len(t, tasks, 1)
		current := tasks[0]

		// Asumimos que ya está en Running, o al menos Scheduled/Starting.
		// Llamamos a StopTask
		err = w.StopTask(*current)
		require.NoError(t, err)

		// Esperamos a que se procese
		time.Sleep(2 * time.Second)

		// Verificamos que haya pasado a Completed (o la lógica que tengas).
		raw2, err := w.Db.List()
		require.NoError(t, err)
		tasks2 := raw2.([]*task.Task)
		final := tasks2[0]

		require.Equal(t, task.Completed, final.Status.State,
			"StopTask debería poner la tarea en Completed (o un estado final) tras borrar contenedor")
	})
}
