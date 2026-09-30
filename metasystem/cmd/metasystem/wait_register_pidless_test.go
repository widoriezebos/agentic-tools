package main

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// session wait --label TEXT --timeout D registers a wait with no pid (the
// seat's in-process sub-agents): the timeout is required and at most 2h.
func TestSessionWaitWithoutAPidNeedsABoundedTimeout(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "no timeout", args: []string{"--label", "sub-agents"}, want: "--timeout"},
		{name: "over two hours", args: []string{"--label", "sub-agents", "--timeout", "3h"}, want: "2 hours"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, _, problem := runOnOwnStreams(func(stdout, stderr io.Writer) int { return runSessionWait(test.args, stdout, stderr) })
			if code != metarun.ExitInvalidWait || !strings.Contains(problem, test.want) {
				t.Fatalf("code=%d stderr=%q", code, problem)
			}
		})
	}
	root, _, mainID, now := waitRegisterCommandFixture(t)
	code, output, problem := runOnOwnStreams(func(stdout, stderr io.Writer) int {
		return runSessionWait([]string{"--root", root, "--label", "in-session sub-agents", "--timeout", "90m", "--json"}, stdout, stderr)
	})
	var row metarun.Waiter
	if err := json.Unmarshal([]byte(output), &row); err != nil || code != 0 || problem != "" ||
		row.Kind != "local" || row.Pid != 0 || row.Label != "in-session sub-agents" || row.MainId != mainID ||
		row.RegisteredBootID != "system-boot" || row.Deadline != now.Add(90*time.Minute).Format(time.RFC3339Nano) {
		t.Fatalf("pidless registration code=%d stdout=%q stderr=%q row=%+v err=%v", code, output, problem, row, err)
	}
}
