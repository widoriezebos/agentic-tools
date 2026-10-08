package plain

import (
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

func TestSharedStopRetainsLaneIdentityThroughDecisionAndStorage(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	opened := "2026-10-08T08:00:00Z"
	stop := Stop{Loop: "lane-gate", Subject: "g1, g2", Attempt: 2, Budget: 2, Decision: "stop", Handoff: "ask lane",
		ProofAttempt: "proof", Tree: "tree", BatchID: "batch", Scope: "gate", Trunk: true, StoppedAt: &opened,
		Required: []string{"metasystem", "landing", "prove", "--gate"}, At: opened, Cause: &Cause{Kind: "environment"}}
	decided := loopstop.Decide(loopstop.Input{Stop: stop})
	if !reflect.DeepEqual(stop, decided) {
		t.Fatalf("shared decision lost lane identity: %+v", decided)
	}
	if err := withLock(install, func() error { return appendLine(stopsPath(install), decided) }); err != nil {
		t.Fatal(err)
	}
	lines, err := readLines[Stop](stopsPath(install))
	if err != nil || len(lines) != 1 || !reflect.DeepEqual(lines[0], stop) {
		t.Fatalf("stored stop lost lane identity: %+v %v", lines, err)
	}
}

func TestSharedStopPreservesLaneRemedyCommands(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		stop Stop
		want string
	}{
		{"required", Stop{Loop: "lane-proof", Handoff: "hold incident", Required: []string{"metasystem", "landing", "prove", "--trunk"}}, "metasystem landing prove --trunk"},
		{"incident", Stop{Loop: "lane-proof", Handoff: "hold incident"}, "metasystem incident list"},
		{"environment", Stop{Loop: "lane-proof", Cause: &Cause{Kind: "environment"}}, "metasystem landing prove"},
		{"trunk", Stop{Loop: "lane-proof", Trunk: true, Cause: &Cause{Kind: "environment"}}, "metasystem landing prove --trunk"},
		{"gate", Stop{Loop: "lane-gate", Trunk: true, Cause: &Cause{Kind: "environment"}}, "metasystem landing prove --gate"},
		{"regeneration", Stop{Loop: "lane-proof", Scope: "regeneration", Cause: &Cause{Kind: "environment"}}, "metasystem landing run"},
		{"barren lane", Stop{Loop: "lane-return", Subject: "lane"}, "metasystem landing run"},
		{"returned goal", Stop{Loop: "lane-return", Subject: "goal", Cause: &Cause{Kind: "other", Goal: "goal"}}, "metasystem landing return goal --cause other --reason TEXT"},
		{"unknown cause", Stop{Loop: "lane-proof", Cause: &Cause{Kind: "invalid", Goal: "goal"}}, "metasystem landing return goal --cause unclassified --reason TEXT"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := row.stop.Command(); got != row.want {
				t.Fatalf("command = %q, want %q", got, row.want)
			}
		})
	}
	stop := Stop{Loop: "lane-return", Subject: "goal", Class: "goal corrections did not converge", Attempt: 2}
	if got := stop.Words(); !strings.Contains(got, stop.Class) || strings.Contains(got, "launches left the lane unchanged") {
		t.Fatalf("goal return was rendered as a barren lane: %q", got)
	}
}
