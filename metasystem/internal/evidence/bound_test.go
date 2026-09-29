package evidence

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

var boundNow = time.Date(2026, 12, 29, 2, 0, 0, 0, time.UTC)

const kib = 1 << 10

// boundBed is one nested installation (<git root>/metasystem) armed on the
// host, and its segment of an evidence root.
type boundBed struct {
	root, gitRoot, installation, home string
	segment                           Segment
}

func newBoundBed(t *testing.T) *boundBed {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bed := &boundBed{root: filepath.Join(base, "evidence"), gitRoot: filepath.Join(base, "checkout"), home: filepath.Join(base, "home", ".metasystem")}
	bed.installation = filepath.Join(bed.gitRoot, "metasystem")
	for _, dir := range []string{filepath.Join(bed.gitRoot, ".git"), filepath.Join(bed.installation, "artifacts", "agents", "jobs"), bed.root, bed.home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A template-layout installation: its state root is itself, found
	// without running git.
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf"), []byte("metasystem.template=true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	facts := diskstore.CheckoutFacts{GitRoot: bed.gitRoot, Installation: bed.installation, RootCommit: "e83c5163316f89bfbde7d9ab23ca2e25604af290", LedgerIdentity: "01J9LEDGER0000000000000000"}
	bed.segment = Segment{Root: bed.root, Git: diskstore.Segment(bed.gitRoot), Installation: diskstore.Segment(bed.installation),
		Context: &Context{Installation: bed.installation, Facts: facts}}
	return bed
}

// chain mirrors a closed chain that ended daysAgo, with payloadKiB of logs.
func (bed *boundBed) chain(t *testing.T, name string, daysAgo int, payloadKiB int, goal string) string {
	t.Helper()
	dir := filepath.Join(bed.root, "agents", bed.segment.Git, name)
	record := map[string]any{"jobId": name, "round": 1, "role": "implementer", "status": "completed", "chainClosed": true, "goalId": goal,
		"endedAt": boundNow.Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339)}
	data, _ := json.Marshal(record)
	returned, _ := json.Marshal(map[string]any{"claimed": map[string]any{"model": "claude-opus-5-5"}, "gaps": []any{"one"}, "whatWasDone": "built the thing\nand more"})
	files := map[string][]byte{
		"manifest.json":          []byte(`{"rootJob":"` + name + `","files":{}}`),
		"brief.md":               []byte("the brief"),
		"jobs/" + name + ".json": data,
		"jobs/" + name + ".log":  make([]byte, payloadKiB*kib),
		"rounds/1/return.json":   returned,
		"rounds/1/raw.out":       []byte(strings.Repeat("transcript ", 100)),
		"capabilities/cap.json":  []byte("{}"),
	}
	for rel, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func (bed *boundBed) bound() Bound {
	return Bound{Now: boundNow}
}

func settingsOf(capKiB int64) PassSettings {
	return PassSettings{CapBytes: capKiB * kib, AgeFloor: 90 * 24 * time.Hour, Values: map[string]string{"evidence.segment-cap-gib": "10"}}
}

func receipts(t *testing.T, segment Segment) []diskstore.DisposalReceipt {
	t.Helper()
	lines, err := diskstore.ReadReceipts(segment.Ledger())
	if err != nil {
		t.Fatal(err)
	}
	return lines
}

// Round B2-3, rule 1: past the bound the machine only reports, by how
// much, with the command pair; it changes nothing and writes no receipt.
func TestBoundOnlyReportsOverTheBoundAndChangesNothing(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	for index, name := range []string{"one", "two", "three"} {
		bed.chain(t, name, 100+index, 400, "g")
	}
	before := snapshot(t, bed.root)
	position := bed.bound().ReportSegment(context.Background(), bed.segment, settingsOf(200))
	if !position.Over || len(position.Commands) < 2 || position.Oldest.IsZero() {
		t.Fatalf("the segment is reported over with the command pair: %+v", position)
	}
	if position.Commands[0] != "metasystem evidence dispose --over-bound --export DIR --preview" || position.Commands[1] != "metasystem evidence dispose --plan ID" {
		t.Fatalf("the literal command pair: %q", position.Commands)
	}
	if after := snapshot(t, bed.root); !equalSnapshots(before, after) || len(receipts(t, bed.segment)) != 0 {
		t.Fatal("the machine changes nothing and writes no receipt")
	}
	rendered := strings.Join(position.Lines(false), "\n")
	if !strings.Contains(rendered, "over the bound: "+bed.segment.Git+" of "+bed.gitRoot) || !strings.Contains(rendered, ", over by ") ||
		!strings.Contains(rendered, "the preview shows what a person can remove") {
		t.Fatalf("the report line: %s", rendered)
	}
	bed.segment.Context.Settings.Values = map[string]string{"evidence.export-dir": "/Volumes/Backup/metasystem-exports"}
	again := bed.bound().ReportSegment(context.Background(), bed.segment, settingsOf(200))
	if again.Commands[0] != "metasystem evidence dispose --over-bound --export /Volumes/Backup/metasystem-exports --preview" {
		t.Fatalf("the directory comes from evidence.export-dir: %q", again.Commands)
	}
}

func TestBoundUnderItsCapReportsNothingToDecide(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	bed.chain(t, "small", 300, 1, "g")
	position := bed.bound().ReportSegment(context.Background(), bed.segment, settingsOf(10_000))
	if position.Over || len(position.Commands) != 0 || len(position.Lines(false)) != 0 {
		t.Fatalf("under the cap there is nothing to decide: %+v", position)
	}
}

func TestBoundWalkThatDoesNotFinishReportsNoPosition(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	bed.chain(t, "old", 300, 400, "g")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	position := bed.bound().ReportSegment(ctx, bed.segment, settingsOf(1))
	if position.Over || !strings.Contains(strings.Join(position.Pending, "\n"), "disk.sweep-budget-sec") {
		t.Fatalf("a walk cut short names the setting to raise and claims no position: %+v", position)
	}
}

func TestOnlyAnItemPastTheAgeFloorIsRemovable(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	young := chainItem(bed.chain(t, "young", 10, 1, "g"))
	old := chainItem(bed.chain(t, "old", 100, 1, "g"))
	unknown := old
	unknown.EndUnknown = "a job record is unreadable"
	for _, check := range []struct {
		item Item
		want bool
	}{{young, false}, {old, true}, {unknown, false}} {
		if got, why := bed.bound().Removable(check.item, 90*24*time.Hour); got != check.want {
			t.Fatalf("%s removable = %v (%s)", check.item.Name, got, why)
		}
	}
}

func TestSegmentIndexRevalidates(t *testing.T) {
	t.Parallel()
	bed := newBoundBed(t)
	facts := bed.segment.Context.Facts
	index, err := RecordSegmentIndex(bed.root, facts, boundNow, diskstore.Syncer{})
	if err != nil || index.GitSegment != bed.segment.Git || index.InstallationSegment != bed.segment.Installation {
		t.Fatalf("index=%+v err=%v", index, err)
	}
	if reason := index.Revalidate(facts); reason != "" {
		t.Fatalf("the same checkout revalidates: %s", reason)
	}
	reused := facts
	reused.RootCommit = "0000000000000000000000000000000000000000"
	if reason := index.Revalidate(reused); !strings.Contains(reason, "checkout path reused") {
		t.Fatalf("a reused path: %q", reason)
	}
	readopted := facts
	readopted.LedgerIdentity = "01KOTHER000000000000000000"
	if reason := index.Revalidate(readopted); !strings.Contains(reason, "ledger identity changed") {
		t.Fatalf("a re-adopted ledger: %q", reason)
	}
	again, err := RecordSegmentIndex(bed.root, readopted, boundNow.Add(time.Hour), diskstore.Syncer{})
	if err != nil || again.LedgerIdentity != facts.LedgerIdentity || !again.FirstSeen.Equal(index.FirstSeen) {
		t.Fatalf("an existing index is kept as it is: %+v %v", again, err)
	}
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			data, _ := os.ReadFile(path)
			files[strings.TrimPrefix(path, root)] = string(data)
		}
		return nil
	})
	return files
}

func equalSnapshots(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}
