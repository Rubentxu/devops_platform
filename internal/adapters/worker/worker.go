package worker

import (
	"context"
	"sync"

	"dev.rubentxu.devops-platform/adapters/grpc/grpc/client"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/worker"
	sandbox "dev.rubentxu.devops-platform/adapters/grpc/sanbox"
	"dev.rubentxu.devops-platform/adapters/grpc/stats"
	"dev.rubentxu.devops-platform/core/ports"

	"errors"
	"fmt"

	"dev.rubentxu.devops-platform/core/domain/task"

	"log"
	"time"

	"github.com/golang-collections/collections/queue"
	"github.com/google/uuid"
)

const DEFAULT_TIMEOUT = 30 * time.Minute

type Worker struct {
	Name           string
	Queue          queue.Queue
	Db             ports.Store
	Stats          *stats.Stats
	TaskCount      int
	GrpcClient     *client.WorkerClient                  // Inyectamos el cliente gRPC
	Observers      map[string][]ports.TaskOutputObserver // Mapa de observadores por taskID
	mu             sync.Mutex                            // Mutex para proteger el acceso al mapa
	monitoredTasks map[string]struct{}
	ctx            context.Context
}

// Añadir un mutex para proteger el acceso a los observadores
var observersMu sync.RWMutex

func New(name string, storeDB ports.Store, grpcClient *client.WorkerClient) *Worker {
	w := Worker{
		Name:           name,
		Queue:          *queue.New(),
		GrpcClient:     grpcClient, // Inyectamos el cliente gRPC
		monitoredTasks: make(map[string]struct{}),
	}
	w.Db = storeDB
	return &w
}

// RegisterObserver registra un observador para una tarea específica
func (w *Worker) RegisterObserver(taskID string, observer ports.TaskOutputObserver) {
	observersMu.Lock()
	defer observersMu.Unlock()

	if w.Observers == nil {
		w.Observers = make(map[string][]ports.TaskOutputObserver)
	}
	w.Observers[taskID] = append(w.Observers[taskID], observer)
}

// NotifyObservers notifica a todos los observadores de una tarea
func (w *Worker) NotifyObservers(taskID string, output *ports.CommandOutput) {
	observersMu.RLock()
	defer observersMu.RUnlock()

	if observers, ok := w.Observers[taskID]; ok {
		for _, observer := range observers {
			go func(obs ports.TaskOutputObserver) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				done := make(chan struct{})
				go func() {
					obs.Notify(output)
					close(done)
				}()

				select {
				case <-done:
					// Notificación completada
				case <-ctx.Done():
					log.Printf("Timeout notifying observer for task %s\n", taskID)
				}
			}(observer)
		}
	}
}

func (w *Worker) CheckTaskHealth(t task.Task) {
	result, err := w.Db.Get(t.Metadata.ID.String())
	if err != nil {
		log.Printf("Error getting task %s from database: %v\n", t.Metadata.ID.String(), err)
		return
	}

	taskPersisted := *result.(*task.Task)
	if taskPersisted.Status.State != task.Running {
		return // Solo monitoreamos tareas en ejecución
	}

	// Obtener el timeout de la configuración de la tarea
	timeout := taskPersisted.Spec.Timeout
	if timeout == 0 {

		timeout = DEFAULT_TIMEOUT // Valor por defecto si no está configurado
	}

	healthChan := make(chan *worker.HealthCheckResponse)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go w.GrpcClient.StartHealthCheck(ctx, taskPersisted.Status.ContainerID, healthChan, 5*time.Second)

	for {
		select {
		case healthResponse, ok := <-healthChan:
			if !ok {
				return // Canal cerrado, salimos
			}
			if !healthResponse.IsHealthy {
				taskPersisted.Status.State = task.Failed
				taskPersisted.Status.Message = "Health check failed"
				w.Db.Put(taskPersisted.Metadata.ID.String(), &taskPersisted)
				w.NotifyObservers(taskPersisted.Metadata.ID.String(), &ports.CommandOutput{
					ProcessID: taskPersisted.Metadata.ID.String(),
					State:     "failed",
					ErrorMsg:  healthResponse.Status,
				})
				return // Terminamos el monitoreo para esta tarea
			}
		case <-ctx.Done():
			// Timeout alcanzado
			taskPersisted.Status.State = task.Failed
			taskPersisted.Status.Message = fmt.Sprintf("Health check timeout (%s)", timeout)
			w.Db.Put(taskPersisted.Metadata.ID.String(), &taskPersisted)
			w.NotifyObservers(taskPersisted.Metadata.ID.String(), &ports.CommandOutput{
				ProcessID: taskPersisted.Metadata.ID.String(),
				State:     "failed",
				ErrorMsg:  fmt.Sprintf("Health check monitoring timeout after %s", timeout),
			})
			return
		}
	}
}

func (w *Worker) GetTasks() []*task.Task {
	taskList, err := w.Db.List()
	if err != nil {
		log.Printf("error getting list of tasks: %v", err)
		return nil
	}

	return taskList.([]*task.Task)
}

func (w *Worker) CollectStats() {
	for {
		log.Println("Collecting stats")
		w.Stats = stats.GetStats()
		w.TaskCount = w.Stats.TaskCount
		time.Sleep(15 * time.Second)
	}
}

func (w *Worker) AddTask(t task.Task) error {
	if t.Metadata.ID == uuid.Nil {
		return errors.New("task ID cannot be empty")
	}

	if !t.Validate() {
		return errors.New("invalid task configuration")
	}

	w.Queue.Enqueue(t)
	return nil
}

func (w *Worker) RunTasks() {
	for {
		if w.Queue.Len() != 0 {
			result := w.runTask()
			if result.Error != nil {
				log.Printf("Error running task: %v\n", result.Error)
			}
		} else {
			log.Printf("No tasks to process currently.\n")
		}
		time.Sleep(10 * time.Second)
	}
}

func (w *Worker) runTask() task.TaskResult {
	t := w.Queue.Dequeue()
	if t == nil {
		log.Println("[worker] No tasks in the queue")
		return task.TaskResult{Error: nil}
	}

	taskQueued := t.(task.Task)
	fmt.Printf("[worker] Found task in queue: %v:\n", taskQueued)

	err := w.Db.Put(taskQueued.Metadata.ID.String(), &taskQueued)
	if err != nil {
		msg := fmt.Errorf("error storing task %s: %v", taskQueued.Metadata.ID.String(), err)
		log.Println(msg)
		return task.TaskResult{Error: msg}
	}

	result, err := w.Db.Get(taskQueued.Metadata.ID.String())
	if err != nil {
		msg := fmt.Errorf("error getting task %s from database: %v", taskQueued.Metadata.ID.String(), err)
		log.Println(msg)
		return task.TaskResult{Error: msg}
	}

	taskPersisted := *result.(*task.Task)

	if taskPersisted.Status.State == task.Completed {
		return w.StopTask(taskPersisted)
	}

	var taskResult task.TaskResult
	if task.ValidStateTransition(taskPersisted.Status.State, taskQueued.Status.State) {
		switch taskQueued.Status.State {
		case task.Scheduled:
			if taskQueued.Status.ContainerID != "" {
				taskResult = w.StopTask(taskQueued)
				if taskResult.Error != nil {
					log.Printf("%v\n", taskResult.Error)
				}
			}
			taskResult = w.StartTask(taskQueued)
		default:
			fmt.Printf("This is a mistake. taskPersisted: %v, taskQueued: %v\n", taskPersisted, taskQueued)
			taskResult.Error = errors.New("We should not get here")
		}
	} else {
		err := fmt.Errorf("Invalid transition from %v to %v", taskPersisted.Status.State, taskQueued.Status.State)
		taskResult.Error = err
		return taskResult
	}
	return taskResult
}

func (w *Worker) StartTask(t task.Task) task.TaskResult {
	// Crear el sandbox dinámicamente según el tipo especificado en la tarea
	sandboxService, err := sandbox.NewContainerSandboxService(t.Spec.SandboxConfig.SandboxType, t.Spec.SandboxConfig)
	if err != nil {
		log.Printf("Error creating sandbox service for task %v: %v\n", t.Metadata.ID, err)
		t.Status.State = task.Failed
		w.Db.Put(t.Metadata.ID.String(), &t)
		return task.TaskResult{Error: err}
	}

	// Ejecutar el comando en el sandbox
	sandboxAddress, err := sandboxService.CreateContainerSandbox(context.Background(), t.Spec.SandboxConfig)
	if err != nil {
		log.Printf("Error running task %v: %v\n", t.Metadata.ID, err)
		t.Status.State = task.Failed
		w.Db.Put(t.Metadata.ID.String(), &t)
		return task.TaskResult{Error: err}
	}

	t.Status.ContainerID = sandboxAddress
	t.Status.State = task.Running
	w.Db.Put(t.Metadata.ID.String(), &t)

	// Crear un contexto con timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute) // Ajusta el timeout según sea necesario
	defer cancel()

	outputChan := make(chan *ports.CommandOutput)
	go func() {
		defer close(outputChan)
		err := w.GrpcClient.ExecuteCommand(ctx, t.Metadata.ID.String(), t.Spec.Command, t.Spec.SandboxConfig.WorkingDir, t.Spec.SandboxConfig.EnvVars, outputChan)
		if err != nil {
			log.Printf("Error executing command for task %v: %v\n", t.Metadata.ID, err)
			w.NotifyObservers(t.Metadata.ID.String(), &ports.CommandOutput{
				ProcessID: t.Metadata.ID.String(),
				State:     "failed",
				ErrorMsg:  err.Error(),
			})
		}
	}()

	go w.processTaskOutput(ctx, t.Metadata.ID.String(), outputChan)

	// Añadir defer para limpieza en caso de error
	defer func() {
		if err != nil {
			if sandboxAddress != "" {
				sandboxService.DeleteContainerSandbox(context.Background(), sandboxAddress)
			}
		}
	}()

	return task.TaskResult{ContainerId: sandboxAddress}
}

func (w *Worker) processTaskOutput(ctx context.Context, taskID string, outputChan <-chan *ports.CommandOutput) {
	for {
		select {
		case output, ok := <-outputChan:
			if !ok {
				return
			}
			w.NotifyObservers(taskID, output)
			w.updateTaskStatus(taskID, output)
		case <-ctx.Done():
			log.Printf("Context done for task %s: %v\n", taskID, ctx.Err())
			return
		}
	}
}

func (w *Worker) updateTaskStatus(taskID string, output *ports.CommandOutput) {
	result, err := w.Db.Get(taskID)
	if err != nil {
		log.Printf("Error getting task %s: %v\n", taskID, err)
		return
	}

	taskResult := result.(*task.Task)
	switch output.State {
	case "completed":
		taskResult.Status.State = task.Completed
	case "failed":
		taskResult.Status.State = task.Failed
	case "running":
		taskResult.Status.State = task.Running
	}

	taskResult.Status.Message = output.ErrorMsg
	if output.ExitCode != 0 {
		taskResult.Status.ExitCode = &output.ExitCode
	}

	w.Db.Put(taskID, taskResult)
}

func (w *Worker) StopTask(t task.Task) task.TaskResult {
	// Crear el sandbox dinámicamente según el tipo especificado en la tarea
	sandboxService, err := sandbox.NewContainerSandboxService(t.Spec.SandboxConfig.SandboxType, t.Spec.SandboxConfig)
	if err != nil {
		log.Printf("Error creating sandbox service for task %v: %v\n", t.Metadata.ID, err)
		return task.TaskResult{Error: err}
	}

	// Detener el sandbox
	err = sandboxService.DeleteContainerSandbox(context.Background(), t.Status.ContainerID)
	if err != nil {
		log.Printf("Error stopping sandbox %v: %v\n", t.Status.ContainerID, err)
		return task.TaskResult{Error: err}
	}

	t.Status.FinishTime = func(t time.Time) *time.Time { return &t }(time.Now().UTC())
	t.Status.State = task.Completed
	w.Db.Put(t.Metadata.ID.String(), &t)
	log.Printf("Stopped and removed sandbox %v for task %v\n", t.Status.ContainerID, t.Metadata.ID)

	return task.TaskResult{}
}

func (w *Worker) UpdateTasks() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tasks, err := w.Db.List()
			if err != nil {
				log.Printf("Error getting tasks: %v\n", err)
				time.Sleep(15 * time.Second)
				continue
			}

			// Verificar si hay nuevas tareas en ejecución que necesiten monitoreo
			for _, t := range tasks.([]*task.Task) {
				if t.Status.State == task.Running {
					// Verificar si ya estamos monitoreando esta tarea
					_, err := w.Db.Get("monitored_" + t.Metadata.ID.String())
					if err != nil {
						// Marcar la tarea como monitoreada
						err = w.Db.Put("monitored_"+t.Metadata.ID.String(), struct{}{})
						if err != nil {
							log.Printf("Error marking task as monitored: %v\n", err)
							continue
						}

						// Iniciar monitoreo para la nueva tarea
						go func(task task.Task) {
							defer func() {
								// Eliminar el marcador cuando termine el monitoreo
								err := w.Db.Delete("monitored_" + task.Metadata.ID.String())
								if err != nil {
									log.Printf("Error removing monitored task: %v\n", err)
								}
							}()
							w.CheckTaskHealth(task)
						}(*t)
					}
				}
			}
		case <-w.ctx.Done():
			return // Salir si el contexto es cancelado
		}
	}
}
