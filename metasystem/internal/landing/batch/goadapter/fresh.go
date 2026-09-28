package goadapter

// FreshExecution makes go test execute every selected test again instead of
// reporting a cached pass: the lane asks for it when a run's purpose is to
// execute, such as the second base run of a red diagnosis.
func (Adapter) FreshExecution() []string { return []string{"-count=1"} }
