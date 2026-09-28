package main

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type argvProber map[int64]identity.Exact

func (prober argvProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	exact, ok := prober[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	return exact, identity.Alive, nil
}

// After an owner restart only this batch's own proof is re-attached: a live
// run of the head goal that is some other proof (its own work prove on its
// seat tree, a run from before the plan, or a launcher writing another
// result) is not this batch's run.
func TestBatchOwnerRestartProbeBindsOnlyTheBatchsOwnRun(t *testing.T) {
	t.Parallel()
	root, planned := t.TempDir(), time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	const id = "01j5x00000000000000000ba31"
	record := batch.Record{Units: []batch.Unit{{GoalID: "goal-a", State: batch.UnitJoined}},
		Proof: &batch.Proof{Status: "planned", Tree: "tip-tree"}, History: []batch.HistoryEntry{{At: planned.Format(time.RFC3339Nano), Verb: "prove", Detail: "planned"}}}
	resultPath := batchProofResultPath(root, id)
	launcher := func(pid int64, argv ...string) identity.Exact {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0), Argv: argv, ArgvKnown: len(argv) > 0}
	}
	for _, test := range []struct {
		name    string
		tree    string
		started time.Time
		exact   identity.Exact
		want    string
	}{
		{"the batch's own run", "tip-tree", planned.Add(time.Second), launcher(41, "metasystem", "internal", "test", "run", "--result", resultPath), batch.RunLive},
		{"its own run, argv unreadable", "tip-tree", planned.Add(time.Second), launcher(41), batch.RunLive},
		{"the head goal's work prove on its seat tree", "seat-tree", planned.Add(time.Second), launcher(41, "metasystem", "work", "prove"), batch.RunDead},
		{"a run from before the plan", "tip-tree", planned.Add(-time.Second), launcher(41), batch.RunDead},
		{"a launcher writing another result", "tip-tree", planned.Add(time.Second), launcher(41, "metasystem", "internal", "test", "run", "--result", root+"/other.json"), batch.RunDead},
	} {
		attempts := []proofrun.Attempt{{AttemptID: "running", GoalID: "goal-a", CandidateTree: test.tree, StartedAt: test.started.Format(time.RFC3339Nano),
			Launcher: proofrun.ProcessIdentity{Pid: 41, PidStartedAt: 41}}}
		got, err := probeBatchProofRun(root, id, record, argvProber{41: test.exact}, func(string) ([]proofrun.Attempt, error) { return attempts, nil })
		if err != nil || got.State != test.want {
			t.Errorf("%s: probed %+v err=%v, want %s", test.name, got, err, test.want)
		}
	}
}
