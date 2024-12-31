package domain

type SandboxType string

const (
	SandboxTypeDocker     SandboxType = "docker"
	SandboxTypeKubernetes SandboxType = "kubernetes"
	SandboxTypeHyperV     SandboxType = "hyperv"
)

type SandboxConfig struct {
	SanboxID    string            // Identificador único del sandbox
	Image       string            // Imagen o plantilla del sandbox
	EnvVars     map[string]string // Variables de entorno
	Metadata    map[string]string // Metadatos adicionales específicos de la tecnología
	SandboxType SandboxType       `json:"sandbox_type"` // Tipo de sandbox
	Resources   ResourceRequests  `json:"resources"`
	Network     NetworkConfig     `json:"network,omitempty"`
	Storage     StorageConfig     `json:"storage,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
}

// ResourceRequests especifica los recursos necesarios para la tarea
type ResourceRequests struct {
	CPU    string `json:"cpu"`    // Unidades de CPU (1.0 = 1 core)
	Memory string `json:"memory"` // Memoria en bytes
	Disk   string `json:"disk"`   // Almacenamiento en bytes
}

// NetworkConfig define la configuración de red
type NetworkConfig struct {
	ExposedPorts map[string]struct{} `json:"exposed_ports,omitempty"` // puertos expuestos
	PortBindings map[string]string   `json:"port_bindings,omitempty"` // mapeo de puertos host:container
}

// StorageConfig define la configuración de almacenamiento
type StorageConfig struct {
	Volumes map[string]string `json:"volumes,omitempty"` // path_host:path_container
}
