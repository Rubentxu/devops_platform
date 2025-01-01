package sandbox

import (
	"context"
	"dev.rubentxu.devops-platform/core/domain"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type HyperVConfig struct {
	VMImagePath string // Ruta a la imagen de la VM
}

type HyperVContainerSandboxService struct {
	vmImagePath string
}

func NewHyperVContainerSandboxService(config HyperVConfig) (*HyperVContainerSandboxService, error) {
	return &HyperVContainerSandboxService{vmImagePath: config.VMImagePath}, nil
}

func (h *HyperVContainerSandboxService) CreateContainerSandbox(ctx context.Context, config *domain.SandboxConfig) (string, error) {
	vmImagePath, ok := config.Metadata["vm_image_path"]
	if !ok {
		return "", fmt.Errorf("missing 'vm_image_path' in metadata")
	}

	vmName := fmt.Sprintf("sandbox-%d", time.Now().Unix())

	cmd := exec.Command("powershell", "-Command", fmt.Sprintf(
		`New-VM -Name "%s" -MemoryStartupBytes %s -VHDPath "%s" -SwitchName "Default Switch"`,
		vmName, config.Resources.Memory, vmImagePath,
	))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to create VM: %v", err)
	}

	cmd = exec.Command("powershell", "-Command", fmt.Sprintf(`Start-VM -Name "%s"`, vmName))
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to start VM: %v", err)
	}

	cmd = exec.Command("powershell", "-Command", fmt.Sprintf(
		`(Get-VMNetworkAdapter -VMName "%s").IpAddresses | Where-Object { $_ -match "\d+\.\d+\.\d+\.\d+" }`,
		vmName,
	))
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get VM IP address: %v", err)
	}
	ipAddress := strings.TrimSpace(string(output))

	return fmt.Sprintf("%s:50051", ipAddress), nil
}

func (h *HyperVContainerSandboxService) DeleteContainerSandbox(ctx context.Context, sandboxID string) error {
	// Detener y eliminar la VM
	cmd := exec.Command("powershell", "-Command", fmt.Sprintf(
		`Stop-VM -Name "%s" -Force; Remove-VM -Name "%s" -Force`,
		sandboxID, sandboxID,
	))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete VM: %v", err)
	}
	return nil
}

func (h *HyperVContainerSandboxService) ListContainerSandboxes(ctx context.Context) ([]string, error) {
	// Listar todas las VMs
	cmd := exec.Command("powershell", "-Command", `Get-VM | Select-Object -ExpandProperty Name`)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list VMs: %v", err)
	}

	vmNames := strings.Split(strings.TrimSpace(string(output)), "\n")
	var sandboxAddresses []string
	for _, vmName := range vmNames {
		// Obtener la dirección IP de cada VM
		cmd = exec.Command("powershell", "-Command", fmt.Sprintf(
			`(Get-VMNetworkAdapter -VMName "%s").IpAddresses | Where-Object { $_ -match "\d+\.\d+\.\d+\.\d+" }`,
			vmName,
		))
		ipOutput, err := cmd.Output()
		if err != nil {
			continue // Ignorar VMs sin IP
		}
		ipAddress := strings.TrimSpace(string(ipOutput))
		sandboxAddresses = append(sandboxAddresses, fmt.Sprintf("%s:50051", ipAddress))
	}

	return sandboxAddresses, nil
}
