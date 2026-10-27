package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func TestLandingLoopStopStaysOpenOnUnchangedFingerprint(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	keeper := lane.AgentState{Barren: 2, BarrenFingerprint: "A"}
	b.seams.Lane = func() (lane.Record, error) { return b.record, nil }
	// A running steward owner avoids its Git-backed fingerprint reader;
	// the keeper's separate probe alternates between absent and running.
	b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: true}, nil }
	tick := func(minute int, wantStops, wantLines, wantQuestions int) {
		t.Helper()
		now := bedNow.Add(time.Duration(minute) * time.Minute)
		b.seams.Now = func() time.Time { return now }
		owner := func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{Alive: minute%2 == 1}, nil }
		b.seams.AgentProbe = owner
		data, err := json.Marshal(keeper)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lane.AgentStatePath(b.home), data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := RecordBarrenStop(b.install, keeper, lane.AgentStatePath(b.home), now, b.seams); err != nil {
			t.Fatal(err)
		}
		if err := SyncPolicyQuestion(b.install, b.seams.Machine, now, b.seams); err != nil {
			t.Fatal(err)
		}
		if err := RefreshLifecycle(b.home, now, b.seams); err != nil {
			t.Fatal(err)
		}
		open, err := OpenStops(b.install)
		if err != nil || len(open) != wantStops {
			t.Fatalf("minute %d: open stops=%+v err=%v", minute, open, err)
		}
		lines, err := readLines[Stop](stopsPath(b.install))
		if err != nil || len(lines) != wantLines {
			t.Fatalf("minute %d: stop records=%+v err=%v", minute, lines, err)
		}
		b.step(now)
		questions, problems := channel.WalkQuestions(b.install)
		if len(problems) != 0 || len(questions) != wantQuestions {
			t.Fatalf("minute %d: questions=%+v problems=%v", minute, questions, problems)
		}
		active := 0
		for _, q := range questions {
			if q.State == "open" {
				active++
			}
			if wantQuestions == 1 && (q.State != "open" || channel.LaneStopCommand(q) == "") {
				t.Fatalf("minute %d: loop question withdrawn or lost: %+v", minute, q)
			}
		}
		if active != 1 {
			t.Fatalf("minute %d: open questions=%d", minute, active)
		}
		status := ObserveStatus(b.home, b.record, b.seams, nil)
		row := healthRow(status.Health, "lane-loop")
		if row == nil || row.Subject != "lane" || !strings.Contains(row.Act, fmt.Sprintf("%d barren runs", keeper.Barren)) {
			t.Fatalf("minute %d: loop row=%+v", minute, row)
		}
		if minute%2 == 1 && status.Owner.Barren != 0 {
			t.Fatalf("running owner view kept barren count: %+v", status.Owner)
		}
	}
	for minute := range 60 {
		tick(minute, 1, 1, 1)
	}
	keeper.BarrenFingerprint = "B"
	tick(60, 1, 3, 2)
	tick(61, 1, 3, 2)
	before, err := ReadStewardLine(b.install)
	if err != nil {
		t.Fatal(err)
	}
	keeper.Barren = 3
	tick(63, 1, 3, 2)
	after, err := ReadStewardLine(b.install)
	if err != nil || before.Identity != after.Identity {
		t.Fatalf("growing barren count changed identity: before=%+v after=%+v err=%v", before, after, err)
	}
	if err := appendRawLine(stopsPath(b.install), Stop{Loop: "lane-proof", Subject: "main", Trunk: true, Decision: "stop", At: bedNow.Add(61 * time.Minute).Format(time.RFC3339Nano)}); err != nil {
		t.Fatal(err)
	}
	tick(64, 2, 4, 2)
	keeper.Barren = 0
	if err := RecordBarrenStop(b.install, keeper, lane.AgentStatePath(b.home), bedNow.Add(65*time.Minute), b.seams); err != nil {
		t.Fatal(err)
	}
	open, err := OpenStops(b.install)
	if err != nil || len(open) != 1 || open[0].Loop != "lane-proof" {
		t.Fatalf("lane progress did not close only the lane stop: %+v %v", open, err)
	}
	if len(b.calls) != 0 {
		t.Fatalf("report-only loop triggered steward acts: %v", b.calls)
	}
}
