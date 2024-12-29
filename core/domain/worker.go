package domain

// Información básica del Worker (nombre, ID, IP, etc.).
type Worker struct {
    ID        string
    Name      string
    IP        string
    IsHealthy bool
    IsBusy    bool
    // Otras métricas o atributos, como CPU, memoria, disco
}
