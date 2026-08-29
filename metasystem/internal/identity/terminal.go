package identity

// ControllingTerminal reports whether a process has a controlling
// terminal — the positive fact that distinguishes a person's shell
// from headless automation. A fixture entry, when present, DECIDES
// (a staged fact must not lose to the machine the suite happens to
// run on); otherwise the kernel answers natively (sysctl on darwin,
// /proc on linux). ok is false when neither source can answer.
func ControllingTerminal(pid int64, probe FixtureProbe) (has bool, ok bool) {
	if entry, present := probeEntry(probe, pid); present && entry.HasTerminal {
		return entry.Terminal, true
	}
	return kernelTerminal(pid)
}

// ControllingTerminalIdentity returns the platform terminal identity for a
// live process. Fixture booleans deliberately cannot satisfy this stronger
// proof because they carry no terminal identity.
func ControllingTerminalIdentity(pid int64) (string, bool) {
	return kernelTerminalIdentity(pid)
}

// ExecutablePath returns the kernel-observed executable for one process.
// Command-line token zero is not a substitute for this fact.
func ExecutablePath(pid int64) (string, bool) {
	return kernelExecutablePath(pid)
}
