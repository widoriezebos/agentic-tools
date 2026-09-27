package delegation

// The lifecycle phases a later U6b commit ports; each refuses until then.

func (s *session) admitCritiqueRead(role, requestingRoot string, round int64, subjectFile, servedGoal string) error {
	panic("delegation: admitCritiqueRead is ported by a later U6b commit")
}

func (s *session) breachStopRun(stopID string) error {
	panic("delegation: breachStopRun is ported by a later U6b commit")
}

func (s *session) cancelJob(args []string) error {
	panic("delegation: cancelJob is ported by a later U6b commit")
}

func (s *session) closeChain(args []string) error {
	panic("delegation: closeChain is ported by a later U6b commit")
}

func (s *session) internalCancel(job string) error {
	panic("delegation: internalCancel is ported by a later U6b commit")
}

func (s *session) parseReapArgs(args []string) (string, string, error) {
	panic("delegation: parseReapArgs is ported by a later U6b commit")
}

func (s *session) reapJobs(args []string) error {
	panic("delegation: reapJobs is ported by a later U6b commit")
}

func (s *session) statusJob(args []string) error {
	panic("delegation: statusJob is ported by a later U6b commit")
}

func usageRootJobID(jobs, job string) (string, error) {
	panic("delegation: usageRootJobID is ported by a later U6b commit")
}

func (s *session) watchJob(args []string) error {
	panic("delegation: watchJob is ported by a later U6b commit")
}

func (s *session) windDownGroup(record string) bool {
	panic("delegation: windDownGroup is ported by a later U6b commit")
}
