package goal

import (
	"strings"
	"testing"
	"time"
)

// A pidless local wait (in-session sub-agents) counts like a pid wait for
// the idle-backlog branch until its deadline; a row longer than the pidless
// bound never counts.
func TestPidlessLocalWaitAllowsTheStopUntilItsDeadline(t *testing.T) {
	t.Parallel()
	fixture := detachedWaitFixture(t, "local", true)
	fixture.row.Pid, fixture.row.PidStartedAt, fixture.row.PidStartedAtMicro, fixture.row.PidStartTicks, fixture.row.BootID = 0, 0, 0, 0, ""
	fixture.writeRow(t)
	verdict := fixture.verdict(t, fixture.scan)
	if verdict.ShouldBlock || !strings.Contains(verdict.Display, "WAITING: compile the release until "+fixture.row.Deadline) {
		t.Fatalf("pending pidless wait verdict=%+v", verdict)
	}

	for name, mutate := range map[string]func(*pendingWaitVerdictFixture){
		"deadline": func(f *pendingWaitVerdictFixture) { f.row.Deadline = f.store.Now().Format(time.RFC3339Nano) },
		"too long": func(f *pendingWaitVerdictFixture) {
			f.row.Deadline = f.store.Now().Add(3 * time.Hour).Format(time.RFC3339Nano)
		},
		"boot": func(f *pendingWaitVerdictFixture) { f.row.RegisteredBootID = "earlier-boot" },
	} {
		expired := detachedWaitFixture(t, "local", true)
		expired.row.Pid, expired.row.PidStartedAt, expired.row.PidStartTicks, expired.row.BootID = 0, 0, 0, ""
		mutate(expired)
		expired.writeRow(t)
		if verdict := expired.verdict(t, expired.scan); !verdict.ShouldBlock || strings.Contains(verdict.Display, "WAITING: compile") {
			t.Fatalf("%s: an invalid pidless wait allowed the stop: %+v", name, verdict)
		}
	}
}
