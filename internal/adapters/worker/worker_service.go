package worker

import (
	"context"
	"dev.rubentxu.devops-platform/core/domain"
	"github.com/google/uuid"
	"sync"

	"dev.rubentxu.devops-platform/adapters/grpc/grpc/client"
	"dev.rubentxu.devops-platform/adapters/grpc/grpc/protos/worker"
	sandbox "dev.rubentxu.devops-platform/adapters/grpc/sandbox"
	"dev.rubentxu.devops-platform/adapters/grpc/stats"
	"dev.rubentxu.devops-platform/core/ports"

	"errors"
	"fmt"

	"dev.rubentxu.devops-platform/core/domain/task"

	"log"
	"time"
)

const DEFAULT_TIMEOUT = 30 * time.Minute

type WorkerService struct {
	Name       string
	Db         ports.Store
	Stats      *stats.Stats
	TaskCount  int
	GrpcClient *client.WorkerClient

	// Observadores (notificaciones de logs, etc.)
	Observers map[string][]ports.TaskOutputObserver

	// Mapa de workers registrados (key=ContainerID)
	workers map[string]domain.WorkerInstance

	// Cola de tareas pendientes
	pendingTasks chan task.Task

	// Límite de concurrencia y contador
	maxConcurrent  int
	currentRunning int
	mu             sync.Mutex

	// Context global
	ctx    context.Context
	cancel context.CancelFunc
}

// Mutex para acceso a Observers
var observersMu sync.RWMutex

func NewWorkerService(
	name string,
	db ports.Store,
	grpcClient *client.WorkerClient,
	parentCtx context.Context,
	queueSize int, // Tamaño de la cola de pendientes
	maxConcurrent int, // Límite de tareas simultáneas
) *WorkerService {
	ctx, cancel := context.WithCancel(parentCtx)

	ws := &WorkerService{
		Name:          name,
		Db:            db,
		GrpcClient:    grpcClient,
		Observers:     make(map[string][]ports.TaskOutputObserver),
		workers:       make(map[string]domain.WorkerInstance),
		pendingTasks:  make(chan task.Task, queueSize),
		maxConcurrent: maxConcurrent,
		ctx:           ctx,
		cancel:        cancel,
	}

	// Lanzar el bucle que consume la cola
	go ws.dispatchLoop()

	return ws
}

// ---------------------------------------------------------
// Bucle despachador (consume de pendingTasks)
// ---------------------------------------------------------
func (w *WorkerService) dispatchLoop() {
	for {
		select {
		case t := <-w.pendingTasks:
			// Hemos sacado una tarea de la cola
			w.mu.Lock()
			if w.currentRunning >= w.maxConcurrent {
				// No hay cupo, "devolvemos" la tarea a la cola
				// (o la dejamos en espera).
				w.mu.Unlock()

				log.Printf("[dispatchLoop] No concurrency slot available. Re-queueing task %s", t.Metadata.ID)
				// Pequeña espera antes de reintentar
				time.Sleep(1 * time.Second)
				// Re-encolamos en otro goroutine para no bloquear
				go func(tt task.Task) {
					w.pendingTasks <- tt
				}(t)
				continue
			}
			// Hay cupo
			w.currentRunning++
			w.mu.Unlock()

			// Ejecutamos la tarea en goroutine
			go w.runTask(t)

		case <-w.ctx.Done():
			log.Println("[dispatchLoop] context done, exiting dispatcher")
			return
		}
	}
}

// runTask se encarga de crear contenedor, pasar a "Scheduled", etc.
// Cuando termina la tarea, libera el slot de concurrencia
func (w *WorkerService) runTask(t task.Task) {
	defer func() {
		// Liberar el slot al final
		w.mu.Lock()
		w.currentRunning--
		w.mu.Unlock()
	}()

	// Lógica para crear el contenedor: "AddTask"
	if err := w.createContainer(t); err != nil {
		log.Printf("[runTask] Error createContainer: %v", err)
	}
}

// ---------------------------------------------------------
// 1) AddTask => en vez de crear contenedor, ahora encola
// ---------------------------------------------------------
func (w *WorkerService) AddTask(t task.Task) error {
	// Asignar un ID si no existe
	if t.Metadata.ID == uuid.Nil {
		t.Metadata.ID = uuid.New()
	}

	// Se espera que la tarea arranque en "Pending" (o un estado válido).
	if t.Status.State == task.Completed || t.Status.State == task.Failed {
		return fmt.Errorf("AddTask: the task is already in a final state (%s)", t.Status.State.String())
	}

	// Encolar la tarea
	log.Printf("[AddTask] Enqueue task %s (state=%s)", t.Metadata.ID, t.Status.State.String())
	w.pendingTasks <- t
	return nil
}

// ---------------------------------------------------------
// 2) Crear contenedor (similar a "AddTask" de antes)
// ---------------------------------------------------------
func (w *WorkerService) createContainer(t task.Task) error {
	// 1) from Pending -> Creating
	if !task.ValidStateTransition(t.Status.State, task.Creating) {
		return fmt.Errorf("createContainer: cannot transition from %s to Creating", t.Status.State)
	}
	t.Status.State = task.Creating
	if err := w.Db.Put(t.Metadata.ID.String(), &t); err != nil {
		return fmt.Errorf("createContainer: error storing task: %w", err)
	}

	// 2) Crear el sandbox
	sandboxService, err := sandbox.NewContainerSandboxService(
		t.Spec.SandboxConfig.SandboxType,
		t.Spec.SandboxConfig,
	)
	if err != nil {
		w.failTask(t, fmt.Sprintf("error creating sandbox service: %v", err))
		return err
	}
	containerID, err := sandboxService.CreateContainerSandbox(context.Background(), &t.Spec.SandboxConfig)
	if err != nil {
		w.failTask(t, fmt.Sprintf("error creating container: %v", err))
		return err
	}

	// 3) from Creating -> Scheduled
	if !task.ValidStateTransition(t.Status.State, task.Scheduled) {
		w.failTask(t, "invalid transition Creating->Scheduled")
		return errors.New("invalid transition Creating->Scheduled")
	}
	t.Status.State = task.Scheduled
	t.Status.ContainerID = containerID
	t.Status.Message = "Container created, waiting for worker registration"
	if err := w.Db.Put(t.Metadata.ID.String(), &t); err != nil {
		return fmt.Errorf("createContainer: error updating to scheduled: %w", err)
	}

	w.NotifyObservers(t.Metadata.ID.String(), &ports.CommandOutput{
		ProcessID: t.Metadata.ID.String(),
		State:     t.Status.State.String(),
	})

	log.Printf("[createContainer] Task %s -> Scheduled (container=%s)", t.Metadata.ID, containerID)
	return nil
}

// ---------------------------------------------------------
// 3) RegisterWorker: se llama cuando el contenedor (worker) se inicia
// ---------------------------------------------------------
func (w *WorkerService) RegisterWorker(worker domain.WorkerInstance) error {
	w.mu.Lock()
	w.workers[worker.ID] = worker
	w.mu.Unlock()

	log.Printf("[RegisterWorker] Worker %s registered, healthy=%t", worker.ID, worker.IsHealthy)

	// Buscar la tarea con containerID == worker.ID
	tasksRaw, err := w.Db.List()
	if err != nil {
		return fmt.Errorf("RegisterWorker: error listing tasks: %w", err)
	}
	tasks := tasksRaw.([]*task.Task)

	var t *task.Task
	for _, candidate := range tasks {
		if candidate.Status.ContainerID == worker.ID {
			t = candidate
			break
		}
	}
	if t == nil {
		log.Printf("[RegisterWorker] No matching task for worker %s", worker.ID)
		return nil
	}

	// Esperamos la tarea en Scheduled
	if t.Status.State != task.Scheduled {
		log.Printf("[RegisterWorker] Task %s in state %s, expected Scheduled", t.Metadata.ID, t.Status.State.String())
		return nil
	}

	if !worker.IsHealthy {
		w.failTask(*t, "Worker not healthy")
		return fmt.Errorf("RegisterWorker: worker not healthy")
	}

	// Pasar a Starting
	if !task.ValidStateTransition(t.Status.State, task.Starting) {
		w.failTask(*t, "cannot transition to Starting")
		return fmt.Errorf("RegisterWorker: invalid transition to Starting")
	}
	t.Status.State = task.Starting
	t.Status.Message = "Worker is healthy, starting task"
	_ = w.Db.Put(t.Metadata.ID.String(), t)

	// Iniciar la ejecución real
	if err := w.StartTask(*t); err != nil {
		return fmt.Errorf("RegisterWorker: StartTask failed: %w", err)
	}

	return nil
}

// ---------------------------------------------------------
// 4) StartTask: from Starting -> Running
// ---------------------------------------------------------
func (w *WorkerService) StartTask(t task.Task) error {
	// Validar transición
	if !task.ValidStateTransition(t.Status.State, task.Running) {
		w.failTask(t, fmt.Sprintf("invalid transition from %s to Running", t.Status.State))
		return errors.New("StartTask: invalid transition to Running")
	}
	t.Status.State = task.Running
	t.Status.Message = "Executing command..."
	if err := w.Db.Put(t.Metadata.ID.String(), &t); err != nil {
		return fmt.Errorf("StartTask: error updating DB: %w", err)
	}

	// Timeout
	timeout := t.Spec.Timeout
	if timeout == 0 {
		timeout = DEFAULT_TIMEOUT
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	outputChan := make(chan *ports.CommandOutput)

	// Ejecutar el comando en goroutine
	go func() {
		defer close(outputChan)
		defer cancel()

		err := w.GrpcClient.ExecuteCommand(
			ctx,
			t.Metadata.ID.String(),
			t.Spec.Command,
			t.Spec.SandboxConfig.WorkingDir,
			t.Spec.SandboxConfig.EnvVars,
			outputChan,
		)
		if err != nil {
			log.Printf("[StartTask] Error executing command for task %s: %v", t.Metadata.ID, err)
			w.NotifyObservers(t.Metadata.ID.String(), &ports.CommandOutput{
				ProcessID: t.Metadata.ID.String(),
				State:     "failed",
				ErrorMsg:  err.Error(),
			})
			w.failTask(t, fmt.Sprintf("command execution error: %v", err))
		}
	}()

	// Procesar la salida
	go w.processTaskOutput(ctx, t.Metadata.ID.String(), outputChan)

	w.NotifyObservers(t.Metadata.ID.String(), &ports.CommandOutput{
		ProcessID: t.Metadata.ID.String(),
		State:     t.Status.State.String(),
	})
	return nil
}

// ---------------------------------------------------------
// processTaskOutput y updateTaskStatus
// ---------------------------------------------------------
func (w *WorkerService) processTaskOutput(
	ctx context.Context,
	taskID string,
	outputChan <-chan *ports.CommandOutput,
) {
	for {
		select {
		case output, ok := <-outputChan:
			if !ok {
				return
			}
			w.NotifyObservers(taskID, output)
			w.updateTaskStatus(taskID, output)

		case <-ctx.Done():
			log.Printf("[processTaskOutput] context done for task %s: %v", taskID, ctx.Err())
			return
		}
	}
}

func (w *WorkerService) updateTaskStatus(taskID string, output *ports.CommandOutput) {
	res, err := w.Db.Get(taskID)
	if err != nil {
		log.Printf("[updateTaskStatus] error getting task %s: %v", taskID, err)
		return
	}
	t := res.(*task.Task)

	oldState := t.Status.State
	switch output.State {
	case "running":
		if task.ValidStateTransition(oldState, task.Running) {
			t.Status.State = task.Running
		}
	case "completed":
		if task.ValidStateTransition(oldState, task.Completed) {
			t.Status.State = task.Completed
			now := time.Now().UTC()
			t.Status.FinishTime = &now
		}
	case "failed":
		if task.ValidStateTransition(oldState, task.Failed) {
			t.Status.State = task.Failed
		}
	default:
		log.Printf("[updateTaskStatus] unknown output state: %s", output.State)
		return
	}

	t.Status.Message = output.ErrorMsg
	if output.ExitCode != 0 {
		t.Status.ExitCode = &output.ExitCode
	}

	if err := w.Db.Put(taskID, t); err != nil {
		log.Printf("[updateTaskStatus] error updating DB for task %s: %v", taskID, err)
	}

	// Si la tarea terminó, podríamos hacer más lógica, etc.
	// Por ejemplo, reintentos si falló, etc.
}

// ---------------------------------------------------------
// 5) StopTask
// ---------------------------------------------------------
func (w *WorkerService) StopTask(t task.Task) error {
	// from Running -> Stopping, etc.
	if !task.ValidStateTransition(t.Status.State, task.Stopping) {
		return fmt.Errorf("StopTask: invalid transition from %s to Stopping", t.Status.State)
	}
	t.Status.State = task.Stopping
	t.Status.Message = "Stopping container..."
	if err := w.Db.Put(t.Metadata.ID.String(), &t); err != nil {
		return fmt.Errorf("StopTask: error updating to stopping: %w", err)
	}

	sandboxService, err := sandbox.NewContainerSandboxService(
		t.Spec.SandboxConfig.SandboxType,
		t.Spec.SandboxConfig,
	)
	if err != nil {
		w.failTask(t, fmt.Sprintf("StopTask: error creating sandbox service: %v", err))
		return err
	}
	if err := sandboxService.DeleteContainerSandbox(context.Background(), t.Status.ContainerID); err != nil {
		w.failTask(t, fmt.Sprintf("StopTask: error deleting sandbox: %v", err))
		return err
	}

	// from Stopping -> Completed
	if !task.ValidStateTransition(t.Status.State, task.Completed) {
		w.failTask(t, "invalid transition from Stopping->Completed")
		return errors.New("StopTask: invalid transition to Completed")
	}

	now := time.Now().UTC()
	t.Status.State = task.Completed
	t.Status.FinishTime = &now
	t.Status.Message = "Container stopped"
	if err := w.Db.Put(t.Metadata.ID.String(), &t); err != nil {
		return fmt.Errorf("StopTask: error updating to completed: %w", err)
	}

	w.NotifyObservers(t.Metadata.ID.String(), &ports.CommandOutput{
		ProcessID: t.Metadata.ID.String(),
		State:     "completed",
	})
	log.Printf("[StopTask] Container %s stopped for task %s", t.Status.ContainerID, t.Metadata.ID)

	// Remover worker del mapa, si queremos
	w.mu.Lock()
	delete(w.workers, t.Status.ContainerID)
	w.mu.Unlock()

	return nil
}

// ---------------------------------------------------------
// failTask (auxiliar para marcar una tarea como Failed)
// ---------------------------------------------------------
func (w *WorkerService) failTask(t task.Task, msg string) {
	if !task.ValidStateTransition(t.Status.State, task.Failed) {
		log.Printf("[failTask] invalid transition from %s to Failed", t.Status.State)
		return
	}
	t.Status.State = task.Failed
	t.Status.Message = msg
	if err := w.Db.Put(t.Metadata.ID.String(), &t); err != nil {
		log.Printf("[failTask] error updating DB: %v", err)
	}

	w.NotifyObservers(t.Metadata.ID.String(), &ports.CommandOutput{
		ProcessID: t.Metadata.ID.String(),
		State:     "failed",
		ErrorMsg:  msg,
	})
}

// ---------------------------------------------------------
// Observadores
// ---------------------------------------------------------
func (w *WorkerService) RegisterObserver(taskID string, obs ports.TaskOutputObserver) {
	observersMu.Lock()
	defer observersMu.Unlock()
	w.Observers[taskID] = append(w.Observers[taskID], obs)
}

func (w *WorkerService) NotifyObservers(taskID string, output *ports.CommandOutput) {
	observersMu.RLock()
	obsList, found := w.Observers[taskID]
	observersMu.RUnlock()

	if !found {
		return
	}
	for _, o := range obsList {
		go func(observer ports.TaskOutputObserver) {
			observer.Notify(output)
		}(o)
	}
}

// ---------------------------------------------------------
// CheckTaskHealth y UpdateTasks, si lo quieres en WorkerService
// ---------------------------------------------------------
func (w *WorkerService) CheckTaskHealth(t task.Task) {
	// Solo si está en Running
	if t.Status.State != task.Running {
		return
	}

	timeout := t.Spec.Timeout
	if timeout == 0 {
		timeout = DEFAULT_TIMEOUT
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	healthChan := make(chan *worker.HealthCheckResponse)
	go w.GrpcClient.StartHealthCheck(ctx, t.Status.ContainerID, healthChan, 5*time.Second)

	for {
		select {
		case resp, ok := <-healthChan:
			if !ok {
				return
			}
			if !resp.IsHealthy {
				w.failTask(t, "health check failed: "+resp.Status)
				return
			}
		case <-ctx.Done():
			w.failTask(t, "health check timeout")
			return
		}
	}
}

func (w *WorkerService) UpdateTasks() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tasksRaw, err := w.Db.List()
			if err != nil {
				log.Printf("[UpdateTasks] error listing tasks: %v", err)
				continue
			}
			tasks := tasksRaw.([]*task.Task)

			for _, t := range tasks {
				if t.Status.State == task.Running {
					go w.CheckTaskHealth(*t)
				}
			}
		case <-w.ctx.Done():
			log.Println("[UpdateTasks] context done, exiting")
			return
		}
	}
}

// ---------------------------------------------------------
// Recopilar estadísticas
// ---------------------------------------------------------
func (w *WorkerService) CollectStats() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.Stats = stats.GetStats()
			w.TaskCount = w.Stats.TaskCount
			log.Printf("[CollectStats] TaskCount=%d", w.TaskCount)
		case <-w.ctx.Done():
			log.Println("[CollectStats] context done, exiting")
			return
		}
	}
}

// ---------------------------------------------------------
// Obtener lista de tareas
// ---------------------------------------------------------
func (w *WorkerService) GetTasks() []*task.Task {
	raw, err := w.Db.List()
	if err != nil {
		log.Printf("[GetTasks] error listing tasks: %v", err)
		return nil
	}
	return raw.([]*task.Task)
}

// ---------------------------------------------------------
// Cerrar el servicio (detener goroutines)
// ---------------------------------------------------------
func (w *WorkerService) Shutdown() {
	w.cancel()
	// Podrías cerrar el canal si quieres descartar tareas pendientes:
	// close(w.pendingTasks)
	// Pero cuidado con enviar más tareas luego de esto.
}
