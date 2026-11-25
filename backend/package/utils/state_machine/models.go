package stateMachineUtils

import "sync"

type State[K comparable] struct{ value K }

type StateMachine[K comparable] struct {
	core *core[K]
}

type core[K comparable] struct {
	mu          sync.RWMutex
	current     State[K]
	transitions map[State[K]][]State[K]
}

type wireMachine[K comparable] struct {
	CurrentState       *State[K]           `json:"currentState"`
	AllowedTransitions []wireTransition[K] `json:"allowedTransitions"`
}

type wireTransition[K comparable] struct {
	From State[K]   `json:"from"`
	To   []State[K] `json:"to"`
}
