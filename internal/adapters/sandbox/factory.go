package sandbox

import (
	"dev.rubentxu.devops-platform/core/domain"
	"dev.rubentxu.devops-platform/core/ports"
	"errors"
	"fmt"
)

func NewContainerSandboxService(sandboxType domain.SandboxType, config interface{}) (ports.ContainerSandboxService, error) {
	switch sandboxType {
	case domain.SandboxTypeDocker:
		return NewDockerContainerSandboxService()
	case domain.SandboxTypeKubernetes:
		kubeConfig, ok := config.(KubernetesConfig)
		if !ok {
			return nil, errors.New("invalid Kubernetes config")
		}
		return NewKubernetesContainerSandboxService(kubeConfig)
	case domain.SandboxTypeHyperV:
		hyperVConfig, ok := config.(HyperVConfig)
		if !ok {
			return nil, errors.New("invalid Hyper-V config")
		}
		return NewHyperVContainerSandboxService(hyperVConfig)
	default:
		return nil, fmt.Errorf("unsupported sandbox type: %s", sandboxType)
	}
}
