package task

import (	
	"log"
)

// State es un enumerado para controlar el ciclo de vida de la tarea
type State int

const (
    Pending State = iota
    Creating
    Scheduled
    Starting
    Running
    Stopping
    Completed
    Failed
)

func AllStates() []string {
    return []string{
        "Pending",
        "Creating",
        "Scheduled",
        "Starting",
        "Running",
        "Stopping",
        "Completed",
        "Failed",
    }
}

func (s State) String() string {
    switch s {
    case Pending:
        return "Pending"
    case Creating:
        return "Creating"
    case Scheduled:
        return "Scheduled"
    case Starting:
        return "Starting"
    case Running:
        return "Running"
    case Stopping:
        return "Stopping"
    case Completed:
        return "Completed"
    case Failed:
        return "Failed"
    default:
        return "Unknown"
    }
}

var stateTransitionMap = map[State][]State{
    Pending:   {Pending, Creating, Scheduled},
    Creating:  {Creating, Scheduled, Failed},
    Scheduled: {Scheduled, Starting, Failed},
    Starting:  {Starting, Running, Failed},
    Running:   {Running, Stopping, Completed, Failed, Scheduled},
    Stopping:  {Stopping, Completed, Failed},
    Completed: {},
    Failed:    {Scheduled},
}

func Contains(states []State, state State) bool {
    for _, s := range states {
        if s == state {
            return true
        }
    }
    return false
}

// ValidStateTransition valida si es posible pasar de src a dst según stateTransitionMap
func ValidStateTransition(src State, dst State) bool {
    log.Printf("attempting to transition from %s to %s\n", src.String(), dst.String())
    return Contains(stateTransitionMap[src], dst)
}


