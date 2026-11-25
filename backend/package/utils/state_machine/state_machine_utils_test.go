package stateMachineUtils_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	sm "github.com/thesahibnanda-max/eCommerce/backend/package/utils/state_machine"
)

type orderStatus string

var (
	pending   = sm.NewState(orderStatus("pending"))
	paid      = sm.NewState(orderStatus("paid"))
	shipped   = sm.NewState(orderStatus("shipped"))
	delivered = sm.NewState(orderStatus("delivered"))
	cancelled = sm.NewState(orderStatus("cancelled"))
)

func orderTransitions() map[sm.State[orderStatus]][]sm.State[orderStatus] {
	return map[sm.State[orderStatus]][]sm.State[orderStatus]{
		pending: {paid, cancelled},
		paid:    {shipped, cancelled},
		shipped: {delivered},
	}
}

func newOrderMachine(t *testing.T) sm.StateMachine[orderStatus] {
	t.Helper()
	machine, err := sm.NewStateMachine(pending, orderTransitions())
	if err != nil {
		t.Fatalf("NewStateMachine() error = %v", err)
	}
	return machine
}

type order struct {
	ID     string                       `json:"id"`
	Status sm.StateMachine[orderStatus] `json:"status"`
}

func TestState(t *testing.T) {
	if pending.Value() != "pending" || pending.IsEmpty() || !(sm.State[orderStatus]{}).IsEmpty() {
		t.Fatal("State basics wrong")
	}
	raw, err := utils.FromObjectToStringJSON(paid)
	if err != nil || raw != `"paid"` {
		t.Fatalf("marshal State = %s, %v", raw, err)
	}
	back, err := utils.FromStringToObjectJSON[sm.State[orderStatus]](raw)
	if err != nil || back != paid {
		t.Fatalf("unmarshal State = %v, %v", back, err)
	}
	num, err := utils.FromObjectToStringJSON(sm.NewState(42))
	if err != nil || num != "42" {
		t.Fatalf("marshal int State = %s, %v", num, err)
	}
}

func TestNewStateMachine_Validation(t *testing.T) {
	empty := sm.State[orderStatus]{}
	tests := []struct {
		name        string
		initial     sm.State[orderStatus]
		transitions map[sm.State[orderStatus]][]sm.State[orderStatus]
		want        error
	}{
		{"empty initial", empty, orderTransitions(), sm.ErrStateEmpty},
		{"empty source", pending, map[sm.State[orderStatus]][]sm.State[orderStatus]{empty: {paid}}, sm.ErrStateEmpty},
		{"empty target", pending, map[sm.State[orderStatus]][]sm.State[orderStatus]{pending: {empty}}, sm.ErrStateEmpty},
		{"duplicate target", pending, map[sm.State[orderStatus]][]sm.State[orderStatus]{pending: {paid, paid}}, sm.ErrDuplicateTransition},
		{"unknown initial", sm.NewState(orderStatus("lost")), orderTransitions(), sm.ErrStateUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := sm.NewStateMachine(tt.initial, tt.transitions); !errors.Is(err, tt.want) {
				t.Fatalf("NewStateMachine() error = %v, want %v", err, tt.want)
			}
		})
	}
	if machine, err := sm.NewStateMachine(delivered, orderTransitions()); err != nil || machine.CurrentState() != delivered {
		t.Fatalf("terminal initial = %v, %v", machine.CurrentState(), err)
	}
	if machine, err := sm.NewStateMachine(pending, nil); err != nil || machine.CurrentState() != pending || machine.TransitionTo(paid) {
		t.Fatalf("no transitions machine = %v, %v", machine.CurrentState(), err)
	}
}

func TestNewStateMachine_CopiesTransitions(t *testing.T) {
	transitions := orderTransitions()
	machine, err := sm.NewStateMachine(pending, transitions)
	if err != nil {
		t.Fatal(err)
	}
	transitions[pending] = append(transitions[pending][:0], delivered)
	if machine.CanTransitionTo(delivered) || !machine.CanTransitionTo(paid) {
		t.Fatal("machine shares caller's transitions")
	}
}

func TestTransitionTo(t *testing.T) {
	machine := newOrderMachine(t)
	if machine.CanTransitionTo(shipped) || machine.TransitionTo(shipped) || machine.CurrentState() != pending {
		t.Fatal("illegal transition allowed")
	}
	if machine.TransitionTo(pending) {
		t.Fatal("self transition allowed without being listed")
	}
	for _, next := range []sm.State[orderStatus]{paid, shipped, delivered} {
		if !machine.CanTransitionTo(next) || !machine.TransitionTo(next) || machine.CurrentState() != next {
			t.Fatalf("transition to %v failed", next.Value())
		}
	}
	if machine.TransitionTo(cancelled) {
		t.Fatal("transition out of terminal state")
	}
}

func TestCopiesShareState(t *testing.T) {
	machine := newOrderMachine(t)
	copied := machine
	if !copied.TransitionTo(paid) || machine.CurrentState() != paid {
		t.Fatal("copy does not share state")
	}
}

func TestZeroValue(t *testing.T) {
	var machine sm.StateMachine[orderStatus]
	if !machine.CurrentState().IsEmpty() || machine.CanTransitionTo(paid) || machine.TransitionTo(paid) {
		t.Fatal("zero machine misbehaves")
	}
	raw, err := utils.FromObjectToStringJSON(machine)
	if err != nil || raw != "null" {
		t.Fatalf("zero marshal = %s, %v", raw, err)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	machine := newOrderMachine(t)
	machine.TransitionTo(paid)
	raw, err := utils.FromObjectToStringJSON(machine)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	want := `{"currentState":"paid","allowedTransitions":[{"from":"paid","to":["shipped","cancelled"]},{"from":"pending","to":["paid","cancelled"]},{"from":"shipped","to":["delivered"]}]}`
	if raw != want {
		t.Fatalf("marshal =\n%s\nwant\n%s", raw, want)
	}
	for i := 0; i < 20; i++ {
		again, _ := utils.FromObjectToStringJSON(machine)
		if again != raw {
			t.Fatal("marshal output is not deterministic")
		}
	}
	back, err := utils.FromStringToObjectJSON[sm.StateMachine[orderStatus]](raw)
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if back.CurrentState() != paid || !back.CanTransitionTo(shipped) || back.CanTransitionTo(delivered) {
		t.Fatalf("decoded machine wrong: %v", back.CurrentState().Value())
	}
	if !back.TransitionTo(shipped) || machine.CurrentState() != paid {
		t.Fatal("decoded machine shares state with original")
	}
	pointer, err := utils.FromStringToObjectJSON[*sm.StateMachine[orderStatus]](raw)
	if err != nil || pointer.CurrentState() != paid {
		t.Fatalf("pointer unmarshal = %v", err)
	}
	pointerRaw, err := utils.FromObjectToStringJSON(pointer)
	if err != nil || pointerRaw != raw {
		t.Fatalf("pointer marshal = %s, %v", pointerRaw, err)
	}
}

func TestJSONEmbeddedInModel(t *testing.T) {
	o := order{ID: "o1", Status: newOrderMachine(t)}
	o.Status.TransitionTo(paid)
	raw, err := utils.FromObjectToStringJSON(o)
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	back, err := utils.FromStringToObjectJSON[order](raw)
	if err != nil {
		t.Fatalf("unmarshal error = %v", err)
	}
	if back.ID != "o1" || back.Status.CurrentState() != paid || !back.Status.TransitionTo(shipped) {
		t.Fatalf("embedded round trip wrong: %+v", back)
	}
	list, err := utils.FromStringToObjectJSON[[]order]("[" + raw + "," + raw + "]")
	if err != nil || len(list) != 2 || list[1].Status.CurrentState() != paid {
		t.Fatalf("slice round trip = %v", err)
	}
	empty, err := utils.FromStringToObjectJSON[order](`{"id":"o2","status":null}`)
	if err != nil || empty.Status.CurrentState() != (sm.State[orderStatus]{}) {
		t.Fatalf("null status = %v", err)
	}
}

func TestJSONIntStates(t *testing.T) {
	machine, err := sm.NewStateMachine(sm.NewState(1), map[sm.State[int]][]sm.State[int]{
		sm.NewState(1): {sm.NewState(2)},
		sm.NewState(2): {sm.NewState(3), sm.NewState(1)},
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := utils.FromObjectToStringJSON(machine)
	if err != nil || raw != `{"currentState":1,"allowedTransitions":[{"from":1,"to":[2]},{"from":2,"to":[3,1]}]}` {
		t.Fatalf("int marshal = %s, %v", raw, err)
	}
	back, err := utils.FromStringToObjectJSON[sm.StateMachine[int]](raw)
	if err != nil || back.CurrentState().Value() != 1 || !back.TransitionTo(sm.NewState(2)) {
		t.Fatalf("int unmarshal = %v", err)
	}
}

func TestUnmarshalJSON_Rejects(t *testing.T) {
	tests := map[string]error{
		`{"allowedTransitions":[]}`:                                                                   sm.ErrJSONInvalid,
		`{"currentState":"","allowedTransitions":[]}`:                                                 sm.ErrStateEmpty,
		`{"currentState":"lost","allowedTransitions":[{"from":"a","to":["b"]}]}`:                      sm.ErrStateUnknown,
		`{"currentState":"a","allowedTransitions":[{"from":"a","to":["b","b"]}]}`:                     sm.ErrDuplicateTransition,
		`{"currentState":"a","allowedTransitions":[{"from":"a","to":["b"]},{"from":"a","to":["c"]}]}`: sm.ErrDuplicateSource,
		`{"currentState":"a","allowedTransitions":[{"from":"","to":["b"]}]}`:                          sm.ErrStateEmpty,
	}
	for input, want := range tests {
		if _, err := utils.FromStringToObjectJSON[sm.StateMachine[orderStatus]](input); !errors.Is(err, want) {
			t.Fatalf("unmarshal %s error = %v, want %v", input, err, want)
		}
	}
	for _, input := range []string{`{`, `[]`, `{"currentState":5,"allowedTransitions":[]}`, `"x"`} {
		if _, err := utils.FromStringToObjectJSON[sm.StateMachine[orderStatus]](input); err == nil {
			t.Fatalf("unmarshal %s accepted", input)
		}
	}
}

func TestUnmarshalJSON_IntoExistingUpdatesCopies(t *testing.T) {
	machine := newOrderMachine(t)
	copied := machine
	if err := machine.UnmarshalJSON([]byte(`{"currentState":"shipped","allowedTransitions":[{"from":"shipped","to":["delivered"]}]}`)); err != nil {
		t.Fatal(err)
	}
	if copied.CurrentState() != shipped || !copied.CanTransitionTo(delivered) {
		t.Fatal("copy did not see unmarshalled state")
	}
	if err := machine.UnmarshalJSON([]byte(`{"currentState":""}`)); err == nil || machine.CurrentState() != shipped {
		t.Fatal("failed unmarshal changed the machine")
	}
}

func TestConcurrentTransitions(t *testing.T) {
	machine := newOrderMachine(t)
	var wg sync.WaitGroup
	wins := make(chan sm.State[orderStatus], 64)
	for i := 0; i < 32; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if machine.TransitionTo(paid) {
				wins <- paid
			}
		}()
		go func() {
			defer wg.Done()
			if machine.TransitionTo(cancelled) {
				wins <- cancelled
			}
			_, _ = utils.FromObjectToStringJSON(machine)
		}()
	}
	wg.Wait()
	close(wins)
	var got []sm.State[orderStatus]
	for w := range wins {
		got = append(got, w)
	}
	final := machine.CurrentState()
	switch {
	case len(got) == 1 && got[0] == cancelled && final == cancelled:
	case len(got) == 2 && got[0] == paid && got[1] == cancelled && final == cancelled:
	case len(got) == 1 && got[0] == paid && final == paid:
	default:
		t.Fatalf("unexpected race outcome: wins=%v final=%v", got, final.Value())
	}
}
