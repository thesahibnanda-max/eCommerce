package stateMachineUtils

import (
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func NewState[K comparable](value K) State[K] {
	return State[K]{value: value}
}

func (s State[K]) Value() K {
	return s.value
}

func (s State[K]) IsEmpty() bool {
	return s == State[K]{}
}

func (s State[K]) MarshalJSON() ([]byte, error) {
	raw, err := utils.FromObjectToStringJSON(s.value)
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

func (s *State[K]) UnmarshalJSON(data []byte) error {
	value, err := utils.FromStringToObjectJSON[K](string(data))
	if err != nil {
		return err
	}
	s.value = value
	return nil
}

func NewStateMachine[K comparable](initialState State[K], allowedTransitions map[State[K]][]State[K]) (StateMachine[K], error) {
	c, err := newCore(initialState, allowedTransitions)
	if err != nil {
		return StateMachine[K]{}, err
	}
	return StateMachine[K]{core: c}, nil
}

func (sm StateMachine[K]) CurrentState() State[K] {
	if sm.core == nil {
		return State[K]{}
	}
	sm.core.mu.RLock()
	defer sm.core.mu.RUnlock()
	return sm.core.current
}

func (sm StateMachine[K]) CanTransitionTo(state State[K]) bool {
	if sm.core == nil {
		return false
	}
	sm.core.mu.RLock()
	defer sm.core.mu.RUnlock()
	return sm.core.allows(state)
}

func (sm StateMachine[K]) TransitionTo(state State[K]) bool {
	if sm.core == nil {
		return false
	}
	sm.core.mu.Lock()
	defer sm.core.mu.Unlock()
	if !sm.core.allows(state) {
		return false
	}
	sm.core.current = state
	return true
}

func (sm StateMachine[K]) MarshalJSON() ([]byte, error) {
	if sm.core == nil {
		return []byte("null"), nil
	}
	sm.core.mu.RLock()
	wire, err := toWire(sm.core)
	sm.core.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	raw, err := utils.FromObjectToStringJSON(wire)
	if err != nil {
		return nil, err
	}
	return []byte(raw), nil
}

func (sm *StateMachine[K]) UnmarshalJSON(data []byte) error {
	if isJSONNull(data) {
		return nil
	}
	wire, err := utils.FromStringToObjectJSON[wireMachine[K]](string(data))
	if err != nil {
		return err
	}
	decoded, err := fromWire(wire)
	if err != nil {
		return err
	}
	if sm.core == nil {
		sm.core = decoded
		return nil
	}
	sm.core.mu.Lock()
	defer sm.core.mu.Unlock()
	sm.core.current = decoded.current
	sm.core.transitions = decoded.transitions
	return nil
}
