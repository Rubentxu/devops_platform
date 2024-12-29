package interfaces

import "dev.rubentxu.devops-platform/core/domain"

// ProcessRepository define la interfaz para persistir y recuperar procesos
type ProcessRepository interface {
	Save(process domain.ProcessStatus) error
	Get(processID string) (domain.ProcessStatus, error)
	Update(process domain.ProcessStatus) error
	Delete(processID string) error
}
