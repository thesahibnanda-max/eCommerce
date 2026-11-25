package stateMachineUtils

const (
	ErrStateEmpty          = Error("stateMachine: state cannot be empty")
	ErrStateUnknown        = Error("stateMachine: current state is not part of the transitions")
	ErrDuplicateTransition = Error("stateMachine: duplicate transition")
	ErrDuplicateSource     = Error("stateMachine: duplicate source state")
	ErrJSONInvalid         = Error("stateMachine: invalid json")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
