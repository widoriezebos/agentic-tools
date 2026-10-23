package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
)

const outsideRecordPath = "metasystem/records/misc/outside-read.md"
const outsideVerdictRefusal = "the reader's record does not end with Verdict: LAND, so nothing was recorded"

type outsideReadBed struct {
	worktreeCollectFixture
	t                              *testing.T
	inv                            *intentInvocation
	deps                           goalBranchReadDependencies
	gates, delegates, publications int
}

func newOutsideReadBed(t *testing.T, changedTest bool) *outsideReadBed {
	t.Helper()
	var f worktreeCollectFixture
	if !changedTest {
		f = newWorktreeCollectFixture(t)
	} else {
		primary, _, _ := goalBranchMainCLIFixtureBelow(t, "m1", ".")
		primary, err := filepath.EvalSymlinks(primary)
		if err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(primary, "metasystem", "code_test.go"), []byte("package fixture\n// original assertion\n"), 0o644)
		goalSyncMutationGit(t, primary, "add", "metasystem/code_test.go")
		goalSyncMutationGit(t, primary, "commit", "-qm", "seed a test")
		goalSyncMutationGit(t, primary, "push", "-q", "upstream", "HEAD:main")
		worktree := filepath.Join(t.TempDir(), "goal-worktree")
		goalSyncMutationGit(t, primary, "worktree", "add", "-q", "-b", "goal/standing-validation", worktree, "HEAD")
		worktree, err = filepath.EvalSymlinks(worktree)
		if err != nil {
			t.Fatal(err)
		}
		exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
		if err != nil || state != identity.Alive {
			t.Fatalf("probe: %s %v", state, err)
		}
		if _, err := lease.AnnounceWithPair(worktree, "outside-read", exact.Pid, exact.StartedAt.Unix(), exact.StartTicks, exact.BootID, "fixture", "fake", "m1"); err != nil {
			t.Fatal(err)
		}
		writeTestingFixtureFile(t, filepath.Join(worktree, "metasystem", "code_test.go"), []byte("package fixture\n// changed assertion\n"), 0o644)
		goalSyncMutationGit(t, worktree, "add", "metasystem/code_test.go")
		code, out, stderr := runOnOwnStreams(func(out, stderr io.Writer) int {
			return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "u1", "--root", worktree}, out, stderr)
		})
		if code != 0 {
			t.Fatalf("unit: %d %s", code, stderr)
		}
		f = worktreeCollectFixture{primary: primary, worktree: worktree, unit: strings.TrimSpace(out)}
		goalSyncMutationGit(t, worktree, "push", "-q", "upstream", "HEAD:refs/heads/goal/standing-validation")
		writeTestingFixtureFile(t, filepath.Join(primary, "metasystem", "memory", "receipts.log"), []byte("seed receipt\nlive receipt\n"), 0o644)
	}
	b := &outsideReadBed{worktreeCollectFixture: f, t: t}
	// The fixture owns the fetch deadlines, so a loaded host cannot turn this
	// proof into a transport timeout (gate red 2026-10-23 19:08).
	b.deps.ProjectionDeadline = neverProjectionDeadline
	b.deps.Gate = func(string) (string, error) { b.gates++; return "green", nil }
	b.deps.Delegate = func(_, _, _, _, _ string) (string, error) {
		b.delegates++
		t.Error("an outside read dispatched a critic")
		return "unexpected-critic", nil
	}
	b.inv = &intentInvocation{
		layout:    stateroot.Layout{GitRoot: f.primary, RepositoryRoot: f.primary, InstallationRoot: stateroottest.Installation(t, f.primary)},
		stateRoot: f.primary,
		input:     intentInput{values: map[string][]string{"goal": {"standing-validation"}, "reader-record": {outsideRecordPath}}},
		owners: intentOwners{dependencies: syncRequestDependencies{endpoint: goal.ResolveEndpoint, projectionDeadline: neverProjectionDeadline}, commandNow: goalCommandNow, resolver: stateroot.NewResolver(fakeTop(f.primary), noExecutable), delivery: &intentDeliveryOwners{
			branchRead: func(args []string) (branch.BranchReadResult, int, error) { return goalBranchReadRun(args, b.deps) },
			publishRead: func(root, id, unit string) (branch.PublishReadResult, error) {
				b.publications++
				return goalBranchPublishReadWithDeadline(root, id, unit, neverProjectionDeadline)
			},
		}},
	}
	b.write(b.validText())
	return b
}
func (b *outsideReadBed) validText() string {
	b.t.Helper()
	digest, err := branch.UnitDigest(b.worktree, b.unit)
	if err != nil {
		b.t.Fatal(err)
	}
	return b.unit + "\n" + digest + "\nVerdict: LAND\n\n"
}
func (b *outsideReadBed) write(text string) {
	b.t.Helper()
	writeTestingFixtureFile(b.t, filepath.Join(b.worktree, filepath.FromSlash(outsideRecordPath)), []byte(text), 0o644)
}
func (b *outsideReadBed) read() intentResult { b.t.Helper(); return b.inv.reviewCommit(b.unit) }
func (b *outsideReadBed) git(root string, args ...string) string {
	b.t.Helper()
	return goalSyncMutationGit(b.t, root, args...)
}
func (b *outsideReadBed) remote() string {
	return b.git(b.primary, "ls-remote", "upstream", "refs/heads/goal/standing-validation")
}
func (b *outsideReadBed) state(root string) string {
	return b.git(root, "rev-parse", "HEAD") + "\n" + b.git(root, "write-tree")
}
func (b *outsideReadBed) assertNoRead(before, remote string) {
	b.t.Helper()
	if got := b.state(b.worktree); got != before || b.remote() != remote || b.publications != 0 {
		b.t.Fatalf("refusal changed branch or index: before %s after %s; publications %d", before, got, b.publications)
	}
	if _, err := os.Stat(filepath.Join(b.worktree, "metasystem/records/reads/standing-validation", b.unit+".json")); !os.IsNotExist(err) {
		b.t.Fatalf("refusal left an attestation: %v", err)
	}
}

func TestWorkReviewRecordsAnOutsideRead(t *testing.T) {
	t.Parallel()
	t.Run("records", func(t *testing.T) {
		t.Parallel()
		b := newOutsideReadBed(t, true)
		before := b.state(b.primary)
		result := b.read()
		if result.Outcome != intentConfirmed || b.delegates != 0 || b.gates != 1 || b.publications != 1 {
			t.Fatalf("record: %+v gates=%d delegates=%d publications=%d", result, b.gates, b.delegates, b.publications)
		}
		tip := b.git(b.worktree, "rev-parse", "HEAD")
		if !strings.HasPrefix(b.remote(), tip) {
			t.Fatal("read was not published")
		}
		if got := b.git(b.primary, "show", tip+":"+outsideRecordPath); got != strings.TrimSpace(b.validText()) {
			t.Fatalf("published record %q", got)
		}
		base := b.git(b.worktree, "rev-parse", "refs/remotes/upstream/main")
		att, err := branch.ValidateAttestation(b.worktree, base, "standing-validation", "u1", b.unit)
		if err != nil || att.Source.Kind != "reader-record" || len(att.TestsChanged) != 1 || att.TestsChanged[0].ReaderWord != "reviewed by reader record "+outsideRecordPath {
			t.Fatalf("attestation %+v: %v", att, err)
		}
		status, err := branch.InspectStatus(b.worktree, base, tip, "standing-validation")
		if err != nil || len(status.Units) != 1 || status.Units[0].ReadState != "read clean" {
			t.Fatalf("status %+v: %v", status, err)
		}
		state, err := intentBranchStateWithDeadline(b.worktree, "standing-validation", neverProjectionDeadline)
		if err != nil || len(state.Sources) != 1 || state.Sources[0] != "reader-record" {
			t.Fatalf("source %+v: %v", state, err)
		}
		if b.state(b.primary) != before || b.git(b.primary, "symbolic-ref", "--short", "HEAD") != "main" {
			t.Fatal("primary HEAD or index changed")
		}
		data, err := os.ReadFile(filepath.Join(b.primary, "metasystem/memory/receipts.log"))
		if err != nil || string(data) != "seed receipt\nlive receipt\n" {
			t.Fatalf("primary receipt %q: %v", data, err)
		}
		remote := b.remote()
		if result = b.read(); result.Outcome != intentUnchanged || b.remote() != remote {
			t.Fatalf("repeat %+v", result)
		}
	})
	t.Run("counts-for-the-lane", func(t *testing.T) {
		t.Parallel()
		b, _, install := plainLaneBed(t, "reader-record", "reader-record")
		code, result := b.do("work", "land", "standing-validation")
		expectOutcome(t, "hand-in", code, result, intentConfirmed)
		entries, err := plain.Entries(install)
		if err != nil || len(entries) != 1 || len(entries[0].Units) != 2 {
			t.Fatalf("hand-built rows %+v: %v", entries, err)
		}
		for i, row := range entries[0].Units {
			if row.Unit != []string{"u1", "u2"}[i] || row.Read != "critic" || row.Counted != 0 {
				t.Fatalf("hand-built row %+v", row)
			}
		}
	})
	for _, mode := range []string{"correct", "correction-after-land", "changed-by-gate"} {
		t.Run("not-land/"+mode, func(t *testing.T) {
			t.Parallel()
			b := newOutsideReadBed(t, false)
			text := strings.ReplaceAll(b.validText(), "Verdict: LAND", "Verdict: CORRECT")
			if mode == "correction-after-land" {
				text = b.validText() + "Correction: this unit remains unread.\n"
			}
			if mode == "changed-by-gate" {
				b.deps.Gate = func(string) (string, error) { b.gates++; b.write(text); return "green", nil }
			} else {
				b.write(text)
			}
			before, remote := b.state(b.worktree), b.remote()
			result := b.read()
			wantGates := 0
			if mode == "changed-by-gate" {
				wantGates = 1
			}
			if result.Outcome != intentRefused || strings.SplitN(result.Summary, "\n", 2)[0] != outsideVerdictRefusal || b.gates != wantGates {
				t.Fatalf("verdict refusal %+v; gates %d", result, b.gates)
			}
			b.assertNoRead(before, remote)
			data, err := os.ReadFile(filepath.Join(b.worktree, outsideRecordPath))
			if err != nil || string(data) != text {
				t.Fatalf("record overwritten %q: %v", data, err)
			}
		})
	}
	t.Run("no-digest", func(t *testing.T) {
		t.Parallel()
		b := newOutsideReadBed(t, false)
		b.write(b.unit + "\nVerdict: LAND\n")
		before, remote := b.state(b.worktree), b.remote()
		result := b.read()
		digest, err := branch.UnitDigest(b.worktree, b.unit)
		want := "the reader's record must name commit " + b.unit + " and change " + digest
		if err != nil || result.Outcome != intentRefused || strings.SplitN(result.Summary, "\n", 2)[0] != want || b.gates != 0 {
			t.Fatalf("missing digest %+v: %v gates=%d", result, err, b.gates)
		}
		data, ok := result.Data.(map[string]any)
		if !ok || data["code"] != branch.ReadInvalidCode {
			t.Fatalf("record refusal lost its refusal code: %+v", result)
		}
		b.assertNoRead(before, remote)
	})
	for _, mode := range []string{"local-branch", "remote-only"} {
		t.Run("not-in-a-goal-worktree/"+mode, func(t *testing.T) {
			t.Parallel()
			b := newOutsideReadBed(t, false)
			b.git(b.worktree, "checkout", "--detach")
			if mode == "remote-only" {
				b.git(b.primary, "branch", "-D", "goal/standing-validation")
			}
			before := b.state(b.primary)
			result := b.read()
			if result.Outcome != intentRefused || result.Summary != "no worktree has the goal/standing-validation branch checked out; nothing was recorded" {
				t.Fatalf("worktree refusal %+v", result)
			}
			next := strings.Join(result.next, " ")
			if mode == "local-branch" && (!strings.Contains(next, "worktree add") || strings.Contains(next, " -b ")) {
				t.Fatalf("existing branch remedy %s", next)
			}
			if mode == "remote-only" && (!strings.Contains(next, "fetch upstream refs/heads/goal/standing-validation:refs/heads/goal/standing-validation") || len(result.Details) != 1 || !strings.Contains(result.Details[0], "worktree add")) {
				t.Fatalf("missing branch remedy %+v", result)
			}
			if b.state(b.primary) != before || b.git(b.primary, "symbolic-ref", "--short", "HEAD") != "main" {
				t.Fatal("refusal moved primary")
			}
		})
	}
	t.Run("later-unit-touches-the-file", func(t *testing.T) {
		t.Parallel()
		b := newOutsideReadBed(t, false)
		writeTestingFixtureFile(t, filepath.Join(b.worktree, "metasystem/code.go"), []byte("package fixture\n// later unit\n"), 0o644)
		b.git(b.worktree, "add", "metasystem/code.go")
		code, _, stderr := runOnOwnStreams(func(out, stderr io.Writer) int {
			return goalBranchTestCommand([]string{"commit", "--goal", "standing-validation", "--kind", "unit", "--unit", "u2", "--root", b.worktree}, out, stderr)
		})
		if code != 0 {
			t.Fatalf("later unit %d: %s", code, stderr)
		}
		b.git(b.worktree, "push", "-q", "upstream", "HEAD:refs/heads/goal/standing-validation")
		if result := b.read(); result.Outcome != intentConfirmed || b.delegates != 0 || b.publications != 1 {
			t.Fatalf("earlier read %+v", result)
		}
	})
	for _, option := range []string{"brief", "model", "retry", "units", "dispositions", "empty", "blank"} {
		t.Run("combinations/"+option, func(t *testing.T) {
			t.Parallel()
			b := newDeliveryBed(t)
			reads, publishes := 0, 0
			b.owners.branchRead = func([]string) (branch.BranchReadResult, int, error) {
				reads++
				return branch.BranchReadResult{}, 0, nil
			}
			b.owners.publishRead = func(string, string, string) (branch.PublishReadResult, error) {
				publishes++
				return branch.PublishReadResult{}, nil
			}
			args := []string{"work", "review", "--commit", strings.Repeat("a", 40), "--goal", "standing-validation"}
			if option == "empty" {
				args = append(args, "--reader-record=")
			} else if option == "blank" {
				args = append(args, "--reader-record", "  ")
			} else {
				value := "P"
				if option == "retry" {
					value = "1"
				}
				args = append(args, "--reader-record", "P", "--"+option, value)
			}
			code, result := b.do(args...)
			want := "--reader-record records an existing read, so it takes no critic options; nothing was done"
			if option == "empty" || option == "blank" {
				want = "the reader's record path is empty; nothing was recorded"
			}
			if code != 2 || result.Outcome != intentRefused || result.Summary != want || reads != 0 || publishes != 0 {
				t.Fatalf("combination %d %+v reads=%d publishes=%d", code, result, reads, publishes)
			}
			if option == "empty" || option == "blank" {
				if result.Next == nil || flagValue(result.Next.Argv, "--reader-record") != "metasystem/records/misc/FILE" || strings.Contains(strings.Join(result.Next.Argv, " "), "--reader-record=") {
					t.Fatalf("empty path retry still contains the invalid option: %+v", result.Next)
				}
			}
		})
	}
	for _, mode := range []string{"critic-started", "sentinel", "sentinel-by-gate", "reserved"} {
		t.Run("refusals-keep-state/"+mode, func(t *testing.T) {
			t.Parallel()
			b := newOutsideReadBed(t, false)
			sentinel := filepath.Join(b.worktree, "metasystem/records/reads/standing-validation", b.unit+".json")
			var recordPath string
			var recordBefore []byte
			switch mode {
			case "critic-started":
				common := b.git(b.worktree, "rev-parse", "--git-common-dir")
				if !filepath.IsAbs(common) {
					common = filepath.Join(b.worktree, common)
				}
				recordPath = filepath.Join(common, "metasystem/goal-reads/standing-validation", b.unit+".json")
				recordBefore = []byte(`{"schemaVersion":1,"goal":"standing-validation","unitCommit":"` + b.unit + `","tree":"` + b.git(b.worktree, "rev-parse", b.unit+"^{tree}") + `","rootJob":"existing-critic"}` + "\n")
				writeTestingFixtureFile(t, recordPath, recordBefore, 0o644)
			case "sentinel":
				writeTestingFixtureFile(t, sentinel, []byte("sentinel\n"), 0o644)
			case "sentinel-by-gate":
				b.deps.Gate = func(string) (string, error) {
					b.gates++
					writeTestingFixtureFile(t, sentinel, []byte("sentinel\n"), 0o644)
					return "green", nil
				}
			case "reserved":
				release, err := b.inv.unitRunner().ReserveMutation(b.worktree, "standing-validation", "other-writer")
				if err != nil {
					t.Fatal(err)
				}
				defer release()
			}
			before, remote := b.state(b.worktree), b.remote()
			result := b.read()
			if b.state(b.worktree) != before || b.remote() != remote || b.publications != 0 || b.delegates != 0 {
				t.Fatalf("refusal changed state %+v", result)
			}
			switch mode {
			case "critic-started":
				data, err := os.ReadFile(recordPath)
				if result.Outcome != intentRefused || !strings.Contains(result.Summary, "already started with a critic") || err != nil || string(data) != string(recordBefore) || b.gates != 0 {
					t.Fatalf("critic record %q %+v: %v", data, result, err)
				}
			case "reserved":
				if result.Outcome != intentInProgress || !strings.Contains(strings.Join(result.next, " "), "work wait run:") || b.gates != 0 {
					t.Fatalf("reservation %+v gates=%d", result, b.gates)
				}
			default:
				data, err := os.ReadFile(sentinel)
				if result.Outcome != intentRefused || !strings.Contains(result.Summary, "an earlier read file is in the way") || err != nil || string(data) != "sentinel\n" {
					t.Fatalf("sentinel %q %+v: %v", data, result, err)
				}
			}
		})
	}
}
