package domain

// Información básica del WorkerInstance (nombre, ID, IP, etc.).
type WorkerInstance struct {
	ID        string
	Name      string
	Address   string
	Port      int32
	IsHealthy bool
	IsBusy    bool
	// Otras métricas o atributos, como CPU, memoria, disco
}
