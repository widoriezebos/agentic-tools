package run

type WaiterStateClass bool

const (
	WaiterStateEnded, WaiterStateInFlight                                            WaiterStateClass = false, true
	WaiterStateRegistering, WaiterStatePending                                                        = "registering", "pending"
	WaiterStateReady, WaiterStateFailed, WaiterStateInterrupted, WaiterStateDeadline                  = "ready", "failed", "interrupted", "deadline"
)

type WaiterStateDefinition struct {
	Name  string
	Class WaiterStateClass
}

var WaiterStates = [...]WaiterStateDefinition{
	{Name: WaiterStateRegistering, Class: WaiterStateInFlight}, {Name: WaiterStatePending, Class: WaiterStateInFlight},
	{Name: WaiterStateReady, Class: WaiterStateEnded}, {Name: WaiterStateFailed, Class: WaiterStateEnded},
	{Name: WaiterStateInterrupted, Class: WaiterStateEnded}, {Name: WaiterStateDeadline, Class: WaiterStateEnded},
}
