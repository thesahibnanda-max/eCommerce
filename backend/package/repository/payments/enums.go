package paymentsRepository

import (
	stateMachineUtils "github.com/thesahibnanda-max/eCommerce/backend/package/utils/state_machine"
)

type (
	Provider    string
	State       string
	Currency    string
	EventSource string
)

const (
	ProviderInstamojo Provider = "INSTAMOJO"
	ProviderSelfUPI   Provider = "SELF_UPI"

	StateCreated         State = "CREATED"
	StateAwaitingPayment State = "AWAITING_PAYMENT"
	StateVerifying       State = "VERIFYING"
	StateSucceeded       State = "SUCCEEDED"
	StateFailed          State = "FAILED"
	StateExpired         State = "EXPIRED"

	CurrencyINR Currency = "INR"

	EventSourceUser     EventSource = "USER"
	EventSourceAdmin    EventSource = "ADMIN"
	EventSourceWebhook  EventSource = "WEBHOOK"
	EventSourceRedirect EventSource = "REDIRECT"
	EventSourcePoll     EventSource = "POLL"
	EventSourceSystem   EventSource = "SYSTEM"
)

var stateTransitions = map[State][]State{
	StateCreated:         {StateAwaitingPayment, StateFailed},
	StateAwaitingPayment: {StateVerifying, StateSucceeded, StateFailed, StateExpired},
	StateVerifying:       {StateSucceeded, StateFailed, StateExpired},
	StateFailed:          {StateSucceeded},
	StateExpired:         {StateSucceeded},
}

func (p Provider) String() string {
	return string(p)
}

func (p Provider) IsValid() bool {
	return p == ProviderInstamojo || p == ProviderSelfUPI
}

func (s State) String() string {
	return string(s)
}

func (s State) IsValid() bool {
	switch s {
	case StateCreated, StateAwaitingPayment, StateVerifying, StateSucceeded, StateFailed, StateExpired:
		return true
	default:
		return false
	}
}

func (s State) IsOpen() bool {
	return s == StateCreated || s == StateAwaitingPayment || s == StateVerifying
}

func (s State) IsClosed() bool {
	return s == StateSucceeded || s == StateFailed || s == StateExpired
}

func (s State) IsFinal() bool {
	return s == StateSucceeded
}

func (c Currency) String() string {
	return string(c)
}

func (c Currency) IsValid() bool {
	return c == CurrencyINR
}

func (e EventSource) String() string {
	return string(e)
}

func (e EventSource) IsValid() bool {
	switch e {
	case EventSourceUser, EventSourceAdmin, EventSourceWebhook, EventSourceRedirect, EventSourcePoll, EventSourceSystem:
		return true
	default:
		return false
	}
}

func NewStateMachine(current State) (stateMachineUtils.StateMachine[State], error) {
	if !current.IsValid() {
		return stateMachineUtils.StateMachine[State]{}, ErrStateInvalid
	}
	graph := make(map[stateMachineUtils.State[State]][]stateMachineUtils.State[State], len(stateTransitions))
	for from, targets := range stateTransitions {
		next := make([]stateMachineUtils.State[State], 0, len(targets))
		for _, to := range targets {
			next = append(next, stateMachineUtils.NewState(to))
		}
		graph[stateMachineUtils.NewState(from)] = next
	}
	return stateMachineUtils.NewStateMachine(stateMachineUtils.NewState(current), graph)
}
