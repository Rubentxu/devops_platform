package usecase

import (
	"dev.rubentxu.devops-platform/core/domain"
	"fmt"
)

// ProcessUseCase define métodos para orquestar el ciclo de vida de los procesos.
type ProcessUseCase interface {
	ScheduleProcess(req domain.ProcessRequest) (domain.ProcessStatus, error)
	UpdateProcessState(processID string, newState domain.ProcessState, exitCode int32, errMsg string) error
}

// Implementación básica que podríamos mejorar con persistencia.
type processUseCaseImpl struct {
	processes map[string]domain.ProcessStatus
}

func NewProcessUseCaseImpl() ProcessUseCase {
	return &processUseCaseImpl{
		processes: make(map[string]domain.ProcessStatus),
	}
}

func (uc *processUseCaseImpl) ScheduleProcess(req domain.ProcessRequest) (domain.ProcessStatus, error) {
	status := domain.ProcessStatus{
		ProcessID: req.ID,
		State:     domain.StatePending,
	}
	uc.processes[req.ID] = status
	return status, nil
}

func (uc *processUseCaseImpl) UpdateProcessState(
	processID string,
	newState domain.ProcessState,
	exitCode int32,
	errMsg string,
) error {
	procStatus, ok := uc.processes[processID]
	if !ok {
		return fmt.Errorf("proceso no encontrado: %s", processID)
	}
	procStatus.State = newState
	procStatus.ExitCode = exitCode
	procStatus.ErrorMsg = errMsg
	uc.processes[processID] = procStatus
	return nil
}
