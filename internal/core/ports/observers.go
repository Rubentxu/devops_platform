package ports

// CommandOutput representa la salida de un comando
type CommandOutput struct {
	ProcessID string
	IsStderr  bool
	Content   string
	State     string
	ExitCode  int32
	ErrorMsg  string
}

// TaskOutputObserver define la interfaz para los observadores de la salida de una tarea
type TaskOutputObserver interface {
	Notify(output *CommandOutput) // Método para notificar la salida
}
