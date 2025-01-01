package domain

type SandboxType string

const (
	SandboxTypeDocker     SandboxType = "docker"
	SandboxTypeKubernetes SandboxType = "kubernetes"
	SandboxTypeHyperV     SandboxType = "hyperv"
)

type SandboxConfig struct {
	SanboxID    string            // Identificador único del sandbox
	Name        string            // Nombre del sandbox
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

type SandboxConfigBuilder struct {
	sandboxConfig SandboxConfig
}

func NewSandboxConfigBuilder() *SandboxConfigBuilder {
	return &SandboxConfigBuilder{
		sandboxConfig: SandboxConfig{
			EnvVars:  make(map[string]string),
			Metadata: make(map[string]string),
			Network: NetworkConfig{
				ExposedPorts: make(map[string]struct{}),
				PortBindings: make(map[string]string),
			},
			Storage: StorageConfig{
				Volumes: make(map[string]string),
			},
		},
	}
}

func (b *SandboxConfigBuilder) WithSandboxID(id string) *SandboxConfigBuilder {
	b.sandboxConfig.SanboxID = id
	return b
}

func (b *SandboxConfigBuilder) WithImage(image string) *SandboxConfigBuilder {
	b.sandboxConfig.Image = image
	return b
}

func (b *SandboxConfigBuilder) WithEnvVar(key, value string) *SandboxConfigBuilder {
	b.sandboxConfig.EnvVars[key] = value
	return b
}

func (b *SandboxConfigBuilder) WithMetadata(key, value string) *SandboxConfigBuilder {
	b.sandboxConfig.Metadata[key] = value
	return b
}

func (b *SandboxConfigBuilder) WithSandboxType(sandboxType SandboxType) *SandboxConfigBuilder {
	b.sandboxConfig.SandboxType = sandboxType
	return b
}

func (b *SandboxConfigBuilder) WithName(name string) *SandboxConfigBuilder {
	b.sandboxConfig.Name = name
	return b
}

func (b *SandboxConfigBuilder) WithResourceRequests(cpu, memory, disk string) *SandboxConfigBuilder {
	b.sandboxConfig.Resources = ResourceRequests{
		CPU:    cpu,
		Memory: memory,
		Disk:   disk,
	}
	return b
}

func (b *SandboxConfigBuilder) WithExposedPort(port string) *SandboxConfigBuilder {
	b.sandboxConfig.Network.ExposedPorts[port] = struct{}{}
	return b
}

func (b *SandboxConfigBuilder) WithPortBinding(hostPort, containerPort string) *SandboxConfigBuilder {
	b.sandboxConfig.Network.PortBindings[containerPort] = hostPort
	return b
}

func (b *SandboxConfigBuilder) WithVolume(hostPath, containerPath string) *SandboxConfigBuilder {
	b.sandboxConfig.Storage.Volumes[hostPath] = containerPath
	return b
}

func (b *SandboxConfigBuilder) WithWorkingDir(workingDir string) *SandboxConfigBuilder {
	b.sandboxConfig.WorkingDir = workingDir
	return b
}

func (b *SandboxConfigBuilder) Build() SandboxConfig {
	return b.sandboxConfig
}
