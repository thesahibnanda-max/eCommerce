package ordersRepository

import (
	stateMachineUtils "github.com/thesahibnanda-max/eCommerce/backend/package/utils/state_machine"
)

type (
	Currency   string
	OrderState string
	Actor      string
)

const (
	IndianNationalRupeeCurrency Currency = "INR"

	OrderStateCreated         OrderState = "CREATED"
	OrderStateDitched         OrderState = "DITCHED"
	OrderStateProcessing      OrderState = "PROCESSING"
	OrderStateApproved        OrderState = "APPROVED"
	OrderStatePacked          OrderState = "PACKED"
	OrderStateStartedShipping OrderState = "STARTED_SHIPPING"
	OrderStateOutForDelivery  OrderState = "OUT_FOR_DELIVERY"
	OrderStateDelivered       OrderState = "DELIVERED"
	OrderStateCancelled       OrderState = "CANCELLED"
	OrderStateReturned        OrderState = "RETURNED"
	OrderStateRefunded        OrderState = "REFUNDED"
	OrderStateFailed          OrderState = "FAILED"
	OrderStateOnHold          OrderState = "ON_HOLD"

	ActorUser    Actor = "USER"
	ActorAdmin   Actor = "ADMIN"
	ActorSystem  Actor = "SYSTEM"
	ActorGateway Actor = "GATEWAY"
)

var orderStates = []OrderState{
	OrderStateCreated,
	OrderStateDitched,
	OrderStateProcessing,
	OrderStateApproved,
	OrderStatePacked,
	OrderStateStartedShipping,
	OrderStateOutForDelivery,
	OrderStateDelivered,
	OrderStateCancelled,
	OrderStateReturned,
	OrderStateRefunded,
	OrderStateFailed,
	OrderStateOnHold,
}

var orderTransitions = map[OrderState][]OrderState{
	OrderStateCreated:         {OrderStateProcessing, OrderStateDitched, OrderStateFailed, OrderStateCancelled},
	OrderStateProcessing:      {OrderStateApproved, OrderStateFailed, OrderStateOnHold},
	OrderStateOnHold:          {OrderStateApproved, OrderStateFailed, OrderStateCancelled},
	OrderStateApproved:        {OrderStatePacked, OrderStateStartedShipping, OrderStateCancelled},
	OrderStatePacked:          {OrderStateStartedShipping, OrderStateCancelled},
	OrderStateStartedShipping: {OrderStateOutForDelivery, OrderStateDelivered, OrderStateReturned},
	OrderStateOutForDelivery:  {OrderStateDelivered, OrderStateReturned},
	OrderStateDelivered:       {OrderStateReturned},
	OrderStateCancelled:       {OrderStateRefunded},
	OrderStateReturned:        {OrderStateRefunded},
}

func OrderStates() []OrderState {
	return append([]OrderState(nil), orderStates...)
}

func ParseOrderState(value string) (OrderState, error) {
	state := OrderState(value)
	if !state.IsValid() {
		return "", ErrStateInvalid
	}
	return state, nil
}

func (o OrderState) IsTerminal() bool {
	return o.IsValid() && len(orderTransitions[o]) == 0
}

func NewOrderStateMachine(current OrderState) (stateMachineUtils.StateMachine[OrderState], error) {
	if !current.IsValid() {
		return stateMachineUtils.StateMachine[OrderState]{}, ErrStateInvalid
	}
	graph := make(map[stateMachineUtils.State[OrderState]][]stateMachineUtils.State[OrderState], len(orderTransitions))
	for from, targets := range orderTransitions {
		next := make([]stateMachineUtils.State[OrderState], 0, len(targets))
		for _, to := range targets {
			next = append(next, stateMachineUtils.NewState(to))
		}
		graph[stateMachineUtils.NewState(from)] = next
	}
	return stateMachineUtils.NewStateMachine(stateMachineUtils.NewState(current), graph)
}

func (c Currency) IsValid() bool {
	return c == IndianNationalRupeeCurrency
}
func (c Currency) String() string {
	return string(c)
}

func (o OrderState) IsValid() bool {
	switch o {
	case OrderStateCreated,
		OrderStateDitched,
		OrderStateProcessing,
		OrderStateApproved,
		OrderStatePacked,
		OrderStateStartedShipping,
		OrderStateOutForDelivery,
		OrderStateDelivered,
		OrderStateCancelled,
		OrderStateReturned,
		OrderStateRefunded,
		OrderStateFailed,
		OrderStateOnHold:
		return true
	default:
		return false
	}
}

func (o OrderState) String() string {
	return string(o)
}

func (a Actor) IsValid() bool {
	switch a {
	case ActorUser, ActorAdmin, ActorSystem, ActorGateway:
		return true
	default:
		return false
	}
}

func (a Actor) String() string {
	return string(a)
}

func AllowedTransitions(from OrderState) []OrderState {
	return append([]OrderState{}, orderTransitions[from]...)
}
