package steward

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestLaneLineReachesTheDigestOncePerIdentity(t *testing.T) {
	t.Parallel()
	f := laneFixture{channelFixture: newUnservedChannelFixture(t), home: t.TempDir()}
	writeLaneRecord(t, f.home, f.root)
	resolve := func(string) (stateroot.Layout, error) {
		layout := stateroot.Layout{GitRoot: f.root}
		reflect.ValueOf(&layout.InstallationRoot).Elem().SetString(f.root)
		return layout, nil
	}
	failAppend := true
	var batches [][]narratordigest.Entry
	appendDigest := func(root string, entries []narratordigest.Entry, now time.Time) error {
		batches = append(batches, append([]narratordigest.Entry(nil), entries...))
		if failAppend {
			return errors.New("digest unavailable")
		}
		return narratordigest.AppendWithLayoutReader(root, entries, now, resolve)
	}
	seams := plain.ProveSeams{
		Git:       func(_ string, args ...string) (string, error) { return "main", nil },
		Machine:   func(string) (string, error) { return "landing", nil },
		Incidents: func(string, string, string) ([]goal.TrunkRedEntry, error) { return nil, nil },
		Load:      func(time.Time) hostload.Sample { return hostload.Sample{Available: true, Cores: 8, Load1m: 2} },
		Steward:   &plain.StewardEffects{Owner: func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{}, nil }, UsageStore: &launch.Store{Root: t.TempDir()}},
	}
	var output strings.Builder
	step := func(now time.Time) plain.StewardLine {
		t.Helper()
		if _, err := plain.StewardStep(f.home, now, seams); err != nil {
			t.Fatal(err)
		}
		line, err := plain.ReadStewardLine(f.root)
		if err != nil {
			t.Fatal(err)
		}
		readLaneSilenceWithDesignChecks(f.home, f.root, now, appendDigest, &output)
		return line
	}
	f.handIn(t, "g", laneSilentT0.Add(-75*time.Minute))
	check := plain.DesignCheck{Goal: "g", Commit: "commit", Verdict: "design-changed", Reason: "the design changed", At: laneSilentT0.Format(time.RFC3339)}
	if err := plain.RecordDesignCheck(f.root, check); err != nil {
		t.Fatal(err)
	}
	first := step(laneSilentT0)
	cursor := filepath.Join(f.root, "artifacts", "agents", "steward", "lane-line-carried.json")
	if _, err := os.Stat(cursor); !os.IsNotExist(err) || !strings.Contains(output.String(), "digest unavailable") || len(batches) != 1 || len(batches[0]) != 2 {
		t.Fatalf("failed append advanced cursor or split entries: err=%v batches=%+v output=%s", err, batches, output.String())
	}
	failAppend = false
	output.Reset()
	step(laneSilentT0)
	line := step(laneSilentT0.Add(30 * time.Minute))
	pending, err := narratordigest.PendingWithLayoutReader(f.root, resolve)
	if err != nil || strings.Count(pending.Message, "source: lane-steward") != 1 || !strings.Contains(pending.Message, first.Identity) {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	if len(batches) != 3 || len(batches[1]) != 2 || len(batches[2]) != 1 || strings.Count(pending.Message, check.Reason) != 1 {
		t.Fatalf("retry or unchanged identity lost design checks: batches=%+v pending=%+v", batches, pending)
	}
	f.handIn(t, "another", laneSilentT0.Add(-90*time.Minute))
	line = step(line.At)
	pending, err = narratordigest.PendingWithLayoutReader(f.root, resolve)
	if err != nil || strings.Count(pending.Message, "source: lane-steward") != 2 {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	if _, err := os.Stat(NarrationPath(f.root)); !os.IsNotExist(err) {
		t.Fatalf("line went to narration: %v", err)
	}
	if output.Len() != 0 {
		t.Fatal(output.String())
	}
	if err := os.Remove(filepath.Join(plain.Dir(f.root), "steward-line.json")); err != nil {
		t.Fatal(err)
	}
	readLaneSilenceWithDesignChecks(f.home, f.root, line.At, appendDigest, &output)
	if !strings.Contains(output.String(), "lane line unreadable:") {
		t.Fatal(output.String())
	}
}
