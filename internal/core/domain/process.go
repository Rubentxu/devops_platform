package domain

// Representa la solicitud de ejecución de un proceso con sus parámetros.
type ProcessRequest struct {
	ID         string
	Command    []string
	WorkingDir string
	EnvVars    map[string]string
}

// Representa la información de salida o error de un proceso.
type ProcessOutput struct {
	ProcessID string
	IsStderr  bool
	Content   string
}

// Representa el estado de un proceso en ejecución (o finalizado).
type ProcessStatus struct {
	ProcessID string
	State     ProcessState
	ExitCode  int32
	ErrorMsg  string
}
