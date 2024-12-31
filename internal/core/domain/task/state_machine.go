package task

import (
	"fmt"
	"log"
)

type State int

const (
	Pending State = iota
	Scheduled
	Running
	Completed
	Failed
)

func (s State) String() []string {
	return []string{"Pending", "Scheduled", "Running", "Completed", "Failed"}
}

var stateTransitionMap = map[State][]State{
	Pending:   []State{Scheduled},
	Scheduled: []State{Scheduled, Running, Failed},
	Running:   []State{Running, Completed, Failed, Scheduled},
	Completed: []State{},
	Failed:    []State{Scheduled},
}

func Contains(states []State, state State) bool {
	for _, s := range states {
		if s == state {
			return true
		}
	}
	return false
}

func ValidStateTransition(src State, dst State) bool {
	log.Printf("attempting to transition from %#v to %#v\n", src, dst)
	return Contains(stateTransitionMap[src], dst)
}

func StateFromString(stateStr string) (State, error) {
	switch stateStr {
	case "Pending":
		return Pending, nil
	case "Scheduled":
		return Scheduled, nil
	case "Running":
		return Running, nil
	case "Completed":
		return Completed, nil
	case "Failed":
		return Failed, nil
	default:
		return -1, fmt.Errorf("invalid state: %s", stateStr)
	}
}
