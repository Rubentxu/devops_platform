package ports

import (
	"context"
	sandbox "dev.rubentxu.devops-platform/core/domain"
)

type ContainerSandboxService interface {
	// CreateContainerSandbox crea un nuevo sandbox con la configuración proporcionada.
	CreateContainerSandbox(ctx context.Context, config sandbox.SandboxConfig) (string, error)
	// DeleteContainerSandbox elimina un sandbox existente.
	DeleteContainerSandbox(ctx context.Context, sandboxID string) error
	// ListContainerSandboxes devuelve una lista de sandboxes activos.
	ListContainerSandboxes(ctx context.Context) ([]string, error)
}
