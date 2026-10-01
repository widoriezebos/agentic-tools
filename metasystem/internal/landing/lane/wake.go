package lane

// Why the landing agent would run now. landing status --json carries it as
// "wake", and the keeper wakes the agent on the very same read, in process:
// no process reads another's text. An idle lane runs no model. The plain
// lane's queue and proofs are the wake source (plain.KeeperWake).

// Wake is why the landing agent would run now. Reasons empty is an idle
// lane. Unread names each source that could not be read and why: an unread
// source is never a reason, and it is shown, never hidden.
type Wake struct {
	Reasons []string `json:"reasons"`
	Unread  []string `json:"unread"`
}

// WakeSources are the reads a wake is built from. Reasons nil reads none,
// so the lane is idle.
type WakeSources struct {
	Reasons func(root string) ([]string, error)
}

// ReadWake reads why the landing agent of the registered lane would run now.
func ReadWake(record Record, sources WakeSources) Wake {
	wake := Wake{Reasons: []string{}, Unread: []string{}}
	if sources.Reasons == nil {
		return wake
	}
	reasons, err := sources.Reasons(record.Root)
	if err != nil {
		wake.Unread = append(wake.Unread, err.Error())
		return wake
	}
	wake.Reasons = append(wake.Reasons, reasons...)
	return wake
}
