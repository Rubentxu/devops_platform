package domain

// Posibles estados del proceso (compartidos en Worker y WorkerManager).
type ProcessState string

const (
    StatePending   ProcessState = "PENDING"
    StateScheduled ProcessState = "SCHEDULED"
    StateRunning   ProcessState = "RUNNING"
    StateCompleted ProcessState = "COMPLETED"
    StateFailed    ProcessState = "FAILED"
    StateBusy      ProcessState = "BUSY"
    StateAborted   ProcessState = "ABORTED"
)
