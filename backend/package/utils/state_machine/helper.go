package stateMachineUtils

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func buildTransitions[K comparable](transitions map[State[K]][]State[K]) (map[State[K]][]State[K], error) {
	built := make(map[State[K]][]State[K], len(transitions))
	for from, targets := range transitions {
		if from.IsEmpty() {
			return nil, ErrStateEmpty
		}
		seen := make(map[State[K]]struct{}, len(targets))
		copied := make([]State[K], 0, len(targets))
		for _, to := range targets {
			if to.IsEmpty() {
				return nil, ErrStateEmpty
			}
			if _, dup := seen[to]; dup {
				return nil, fmt.Errorf("%w: %v -> %v", ErrDuplicateTransition, from.value, to.value)
			}
			seen[to] = struct{}{}
			copied = append(copied, to)
		}
		built[from] = copied
	}
	return built, nil
}

func isKnown[K comparable](state State[K], transitions map[State[K]][]State[K]) bool {
	if len(transitions) == 0 {
		return true
	}
	if _, ok := transitions[state]; ok {
		return true
	}
	for _, targets := range transitions {
		for _, to := range targets {
			if to == state {
				return true
			}
		}
	}
	return false
}

func newCore[K comparable](current State[K], transitions map[State[K]][]State[K]) (*core[K], error) {
	if current.IsEmpty() {
		return nil, ErrStateEmpty
	}
	built, err := buildTransitions(transitions)
	if err != nil {
		return nil, err
	}
	if !isKnown(current, built) {
		return nil, fmt.Errorf("%w: %v", ErrStateUnknown, current.value)
	}
	return &core[K]{current: current, transitions: built}, nil
}

func toWire[K comparable](c *core[K]) (wireMachine[K], error) {
	type keyed struct {
		key        string
		transition wireTransition[K]
	}
	entries := make([]keyed, 0, len(c.transitions))
	for from, targets := range c.transitions {
		key, err := utils.FromObjectToStringJSON(from.value)
		if err != nil {
			return wireMachine[K]{}, err
		}
		entries = append(entries, keyed{key: key, transition: wireTransition[K]{From: from, To: append([]State[K]{}, targets...)}})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	wire := wireMachine[K]{CurrentState: &c.current, AllowedTransitions: make([]wireTransition[K], 0, len(entries))}
	for _, entry := range entries {
		wire.AllowedTransitions = append(wire.AllowedTransitions, entry.transition)
	}
	return wire, nil
}

func fromWire[K comparable](wire wireMachine[K]) (*core[K], error) {
	if wire.CurrentState == nil {
		return nil, fmt.Errorf("%w: currentState is required", ErrJSONInvalid)
	}
	transitions := make(map[State[K]][]State[K], len(wire.AllowedTransitions))
	for _, t := range wire.AllowedTransitions {
		if _, dup := transitions[t.From]; dup {
			return nil, fmt.Errorf("%w: %v", ErrDuplicateSource, t.From.value)
		}
		transitions[t.From] = t.To
	}
	return newCore(*wire.CurrentState, transitions)
}

func isJSONNull(data []byte) bool {
	return bytes.Equal(bytes.TrimSpace(data), []byte("null"))
}

func (c *core[K]) allows(state State[K]) bool {
	for _, to := range c.transitions[c.current] {
		if to == state {
			return true
		}
	}
	return false
}
