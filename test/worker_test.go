package test

import (
	"dev.rubentxu.devops-platform/adapters/grpc/store"
	"dev.rubentxu.devops-platform/adapters/grpc/worker"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"dev.rubentxu.devops-platform/core/domain/task"
)

func TestWorkerWithInMemoryStore(t *testing.T) {

	// Crear el store usando la factory
	taskStore, err := store.NewStore("memory", "memoryDb-test")
	require.NoError(t, err)

	// Crear el worker inyectando el store
	w := worker.New("test-worker", taskStore, nil)

	// Test 1: Añadir y ejecutar una tarea básica
	t.Run("Execute Basic Task", func(t *testing.T) {
		testTask := task.Task{
			Metadata: task.TaskMetadata{
				ID:        uuid.New(),
				Name:      "test-task",
				CreatedAt: time.Now(),
			},
			Spec: task.TaskSpec{
				Command: []string{"echo", "Hello World"},
				Timeout: 30 * time.Second,
			},
		}

		err := w.AddTask(testTask)
		require.NoError(t, err)

		go w.RunTasks()

		// Esperar a que la tarea se complete
		// Esperar a que la tarea se complete
		time.Sleep(2 * time.Second)

		// Verificar el estado de la tarea
		tasksInterface, err := taskStore.List()
		require.NoError(t, err)

		tasks, ok := tasksInterface.([]task.Task)
		require.True(t, ok, "Failed to cast tasksInterface to []task.Task")
		require.Len(t, tasks, 1)
		require.Equal(t, task.Completed, tasks[0].Status.State)
	})

	// Test 2: Manejo de timeout
	t.Run("Task Timeout", func(t *testing.T) {
		testTask := task.Task{
			Metadata: task.TaskMetadata{
				ID:        uuid.New(),
				Name:      "timeout-task",
				CreatedAt: time.Now(),
			},
			Spec: task.TaskSpec{
				Command: []string{"sleep", "60"},
				Timeout: 2 * time.Second,
			},
		}

		err := w.AddTask(testTask)
		require.NoError(t, err)

		go w.RunTasks()

		// Esperar a que la tarea falle por timeout
		time.Sleep(5 * time.Second)

		// Verificar el estado de la tarea
		tasksInterface, err := taskStore.List()
		require.NoError(t, err)

		tasks, ok := tasksInterface.([]task.Task)
		require.True(t, ok, "Failed to cast tasksInterface to []task.Task")
		require.Len(t, tasks, 2)
		require.Equal(t, task.Failed, tasks[1].Status.State)
		require.Contains(t, tasks[1].Status.Message, "timeout")
	})

	// Test 3: Manejo de errores
	t.Run("Task Error Handling", func(t *testing.T) {
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
		}

		err := w.AddTask(testTask)
		require.NoError(t, err)

		go w.RunTasks()

		// Esperar a que la tarea falle
		time.Sleep(2 * time.Second)

		// Verificar el estado de la tarea
		tasksInterface, err := taskStore.List()
		require.NoError(t, err)

		tasks, ok := tasksInterface.([]task.Task)
		require.True(t, ok, "Failed to cast tasksInterface to []task.Task")
		require.NoError(t, err)
		require.Len(t, tasks, 3)
		require.Equal(t, task.Failed, tasks[2].Status.State)
	})
}
