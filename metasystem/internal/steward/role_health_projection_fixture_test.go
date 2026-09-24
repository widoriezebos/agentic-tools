package steward

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// roleHealthProjectionBed serves rendered ledger bytes through goal.Project.
// Its repository only implements the read operations used by that projection.
type roleHealthProjectionBed struct {
	t     *testing.T
	root  string
	now   time.Time
	paths []string
}

type roleHealthRepository struct {
	goal.Repository
	bed *roleHealthProjectionBed
}

func newRoleHealthProjectionBed(t *testing.T, now time.Time, goals map[string]*goal.GoalFile, extra map[string][]byte) *roleHealthProjectionBed {
	t.Helper()
	b := &roleHealthProjectionBed{t: t, root: t.TempDir(), now: now}
	if err := os.WriteFile(filepath.Join(b.root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rootRecord := &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1}
	b.write("plans/goals/backlog.md", goal.RenderRoot(rootRecord))
	for id, file := range goals {
		b.write("plans/goals/"+id+".md", goal.RenderFile(file))
	}
	for path, data := range extra {
		b.write(path, data)
	}
	return b
}

func (b *roleHealthProjectionBed) write(relative string, data []byte) {
	b.t.Helper()
	path := filepath.Join(b.root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		b.t.Fatal(err)
	}
	for _, existing := range b.paths {
		if existing == relative {
			return
		}
	}
	b.paths = append(b.paths, relative)
}

func (b *roleHealthProjectionBed) project() (goal.Projection, error) {
	b.t.Helper()
	return goal.Project(goal.Endpoint{Root: b.root, Remote: "local", Branch: goal.LocalLedgerBranch, Repository: roleHealthRepository{bed: b}}, false, b.now)
}

func (b *roleHealthProjectionBed) stopCapability(machine string) RoleVerdict {
	b.t.Helper()
	projection, err := b.project()
	if err != nil {
		b.t.Fatal(err)
	}
	reads := 0
	role := checkStopCapabilityEpochFromProjection(b.root, b.now, projection, nil, func(root string) (string, error) {
		if root != b.root {
			b.t.Fatalf("machine read root = %q, want %q", root, b.root)
		}
		reads++
		return machine, nil
	})
	if reads != 1 {
		b.t.Fatalf("machine reads = %d, want 1", reads)
	}
	return role
}

func newRoleTrunkRedBed(t *testing.T, now time.Time, entries []goal.TrunkRedEntry, cadence *goal.CadenceStatus) *roleHealthProjectionBed {
	t.Helper()
	return newRoleHealthProjectionBed(t, now, nil, map[string][]byte{
		"plans/goals/trunk-red.json": roleHealthTrunkRedBytes(t, entries, cadence),
	})
}

func (b *roleHealthProjectionBed) trunkRed(resolve func(string, string, func() time.Time) (config.BatchLanding, error)) RoleVerdict {
	b.t.Helper()
	projection, err := b.project()
	if err != nil {
		b.t.Fatal(err)
	}
	return checkTrunkRedFromProjection(b.root, b.now, projection, nil, resolve)
}

func (b *roleHealthProjectionBed) trunkRedWithoutBatch() RoleVerdict {
	b.t.Helper()
	return b.trunkRed(func(string, string, func() time.Time) (config.BatchLanding, error) {
		b.t.Fatal("batch settings were resolved without a configured batch root")
		return config.BatchLanding{}, nil
	})
}

func (r roleHealthRepository) Accepted() (string, bool, error) {
	return "health-fixture-tip", true, nil
}

func (r roleHealthRepository) Files(commit string, prefixes ...string) (map[string][]byte, error) {
	r.bed.t.Helper()
	if commit != "health-fixture-tip" || len(prefixes) != 2 || prefixes[0] != "plans/goals/" || prefixes[1] != "records/goals/" {
		r.bed.t.Fatalf("unexpected projection read: commit=%q prefixes=%v", commit, prefixes)
	}
	files := make(map[string][]byte, len(r.bed.paths))
	for _, relative := range r.bed.paths {
		if !strings.HasPrefix(relative, prefixes[0]) && !strings.HasPrefix(relative, prefixes[1]) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(r.bed.root, filepath.FromSlash(relative)))
		if err != nil {
			return nil, fmt.Errorf("read fixture ledger %s: %w", relative, err)
		}
		files[relative] = data
	}
	return files, nil
}

func (r roleHealthRepository) CommitTime(commit string) (time.Time, error) {
	if commit != "health-fixture-tip" {
		r.bed.t.Fatalf("unexpected commit time read: %s", commit)
	}
	return r.bed.now, nil
}

func roleHealthTrunkRedBytes(t *testing.T, entries []goal.TrunkRedEntry, cadence *goal.CadenceStatus) []byte {
	t.Helper()
	if entries == nil {
		entries = []goal.TrunkRedEntry{}
	}
	data, err := json.MarshalIndent(map[string]any{"schema": 1, "entries": entries, "cadence": cadence}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}
