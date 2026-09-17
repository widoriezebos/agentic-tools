package batch

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func landingBed(t *testing.T) (assemblyBed, Store) {
	t.Helper()
	bed := assemblyFixture(t)
	prefixes, err := assembleUnits(bed.root, bed.base, bed.record.Units)
	must(t, err)
	bed.record.State, bed.record.PrefixTrees, bed.record.TipTree = StateLanding, prefixes, prefixes[len(prefixes)-1]
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip-attempt"}
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
	bed.record.Receipts = map[string]PrefixReceipt{"goal-a": {GoalID: "goal-a", Tree: prefixes[0], AttemptID: "prefix-attempt"}}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store
}

func greenLandSeams(events *[]string) LandSeams {
	return LandSeams{
		Apply:         func(unit Unit) error { *events = append(*events, "apply:"+unit.GoalID); return nil },
		AppendReceipt: func(unit Unit, _ PrefixReceipt) error { *events = append(*events, "receipt:"+unit.GoalID); return nil },
		Commit: func(unit Unit, _ PrefixReceipt) (string, error) {
			*events = append(*events, "commit:"+unit.GoalID)
			return "commit-" + unit.GoalID, nil
		},
		Held:    func(_, _ string) error { *events = append(*events, "held"); return nil },
		Push:    func(_, _ string) error { *events = append(*events, "push"); return nil },
		Reset:   func(string) error { *events = append(*events, "reset"); return nil },
		Cleanup: func() error { *events = append(*events, "cleanup"); return nil },
	}
}

func TestBatchLandingTransportRunsWholeSeriesOnce(t *testing.T) {
	_, store := landingBed(t)
	var events []string
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), greenLandSeams(&events)))
	want := []string{"apply:goal-a", "receipt:goal-a", "commit:goal-a", "apply:goal-b", "receipt:goal-b", "commit:goal-b", "held", "push", "cleanup"}
	if !slices.Equal(events, want) {
		t.Fatalf("landing events=%v, want %v", events, want)
	}
	if progress := load(t, store).Landing; progress == nil || !progress.HeldChecked || !progress.PushComplete {
		t.Fatalf("landing progress=%+v", progress)
	}
}

func TestBatchLandingResumeRebuildsCompleteSeries(t *testing.T) {
	_, store := landingBed(t)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Landing = &LandingProgress{Base: record.BaseTree, Commits: map[string]string{"goal-a": "lost-local-commit"}}
		return nil
	}))
	var events []string
	seams := greenLandSeams(&events)
	seams.Prepare = func(string) error { events = append(events, "prepare"); return nil }
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams))
	want := []string{"prepare", "apply:goal-a", "receipt:goal-a", "commit:goal-a", "apply:goal-b", "receipt:goal-b", "commit:goal-b", "held", "push", "cleanup"}
	if !slices.Equal(events, want) {
		t.Fatalf("resumed landing events=%v, want %v", events, want)
	}
}

func TestBatchLandingDoesNotRepeatRefusedPushOnUnchangedOrigin(t *testing.T) {
	_, store := landingBed(t)
	pushes := 0
	seams := greenLandSeams(&[]string{})
	seams.Push = func(_, _ string) error {
		pushes++
		return errors.New("origin main moved")
	}
	if err := LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams); err == nil {
		t.Fatal("first moved-origin push was accepted")
	}
	if err := LandSeries(store, testBatchID, "owner", time.Unix(5, 0), seams); err != nil {
		t.Fatalf("unchanged moved origin did not reopen: %v", err)
	}
	record := load(t, store)
	if pushes != 1 || record.State != StateOpen || record.Landing != nil {
		t.Fatalf("unchanged origin pushes=%d record=%+v, want one refused attempt and an open batch", pushes, record)
	}
}

func TestBatchLandingPersistsRecoveryBranchOnSecondRefusal(t *testing.T) {
	_, store := landingBed(t)
	pushes, recoveries, origins := 0, 0, 0
	seams := greenLandSeams(&[]string{})
	seams.PublishBranch = func(string, string) error { return nil }
	seams.Origin = func() (string, error) {
		origins++
		if origins > 2 {
			return "newest-origin", nil
		}
		return "moved-origin", nil
	}
	seams.Push = func(_, _ string) error { pushes++; return errors.New("first endpoint refusal") }
	var recoveryTips []string
	seams.RecoverPush = func(origin, _, tip string) (PushRecovery, error) {
		recoveries++
		recoveryTips = append(recoveryTips, tip)
		if recoveries == 1 {
			return PushRecovery{Origin: "newest-origin", Tip: "rebased-tip"}, errors.New("second endpoint refusal")
		}
		return PushRecovery{Origin: origin, Tip: "landed-tip", Pushed: true}, nil
	}
	if err := LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams); err == nil {
		t.Fatal("second endpoint refusal was accepted")
	}
	progress := load(t, store).Landing
	if progress == nil || progress.RefusedOrigin != "newest-origin" || progress.BranchTip != "rebased-tip" {
		t.Fatalf("second refusal progress=%+v", progress)
	}
	if err := LandSeries(store, testBatchID, "owner", time.Unix(5, 0), seams); err != nil {
		t.Fatalf("moved origin did not resume from the persisted recovery branch: %v", err)
	}
	progress = load(t, store).Landing
	if pushes != 1 || recoveries != 2 || !slices.Equal(recoveryTips, []string{"commit-goal-b", "rebased-tip"}) || progress == nil || progress.PushedTip != "landed-tip" {
		t.Fatalf("pushes=%d recoveries=%d tips=%v progress=%+v", pushes, recoveries, recoveryTips, progress)
	}
}

func TestBatchLandingOpensAfterThreeRecoveryPushRounds(t *testing.T) {
	bed, store := landingBed(t)
	refusals, originReads := 0, 0
	seams := greenLandSeams(&[]string{})
	seams.Origin = func() (string, error) {
		originReads++
		return fmt.Sprintf("origin-%d", originReads), nil
	}
	seams.Push = func(_, _ string) error {
		refusals++
		return errors.New("initial endpoint refusal")
	}
	seams.RecoverPush = func(origin, _, _ string) (PushRecovery, error) {
		refusals++
		return PushRecovery{Origin: origin, BaseTree: bed.moved, Tip: fmt.Sprintf("rebased-%d", refusals)}, errors.New("recovery endpoint refusal")
	}
	for tick := 0; tick < 4 && load(t, store).State == StateLanding; tick++ {
		_ = LandSeries(store, testBatchID, "owner", time.Unix(int64(4+tick), 0), seams)
	}
	record := load(t, store)
	if refusals != 4 || record.State != StateOpen || record.BaseTree != bed.moved || record.Landing != nil {
		t.Fatalf("refusals=%d record=%+v, want four bounded refusals followed by reopen", refusals, record)
	}
}

func TestBatchLandingResumesPendingMovedOriginRecovery(t *testing.T) {
	_, store := landingBed(t)
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Landing = &LandingProgress{Base: record.BaseTree, Commits: map[string]string{"goal-a": "commit-goal-a", "goal-b": "commit-goal-b"}, BranchTip: "commit-goal-b", HeldChecked: true, RefusedOrigin: "moved-origin", RefusedBase: "older-origin"}
		return nil
	}))
	pushes, recoveries := 0, 0
	seams := greenLandSeams(&[]string{})
	seams.Origin = func() (string, error) { return "moved-origin", nil }
	seams.Push = func(_, _ string) error { pushes++; return errors.New("old series repeated") }
	seams.RecoverPush = func(origin, _, _ string) (PushRecovery, error) {
		recoveries++
		return PushRecovery{Origin: origin, Tip: "rebased-tip", Pushed: true}, nil
	}
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(5, 0), seams))
	record := load(t, store)
	if record.State != StateLanding || record.Landing == nil || record.Landing.PushedTip != "rebased-tip" || pushes != 0 || recoveries != 1 {
		t.Fatalf("resumed record=%+v pushes=%d recoveries=%d", record, pushes, recoveries)
	}
}

func TestBatchLandingMovedInputReturnsOpenOnNewBase(t *testing.T) {
	bed, store := landingBed(t)
	seams := greenLandSeams(&[]string{})
	seams.Origin = func() (string, error) { return "origin-moved", nil }
	seams.Push = func(_, _ string) error { return errors.New("endpoint lease refused") }
	seams.RecoverPush = func(origin, _, _ string) (PushRecovery, error) {
		return PushRecovery{Origin: origin, BaseTree: bed.moved, Reopen: true}, nil
	}
	if err := LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams); err != nil {
		t.Fatal(err)
	}
	record := load(t, store)
	if record.State != StateOpen || record.BaseTree != bed.moved || record.Proof != nil || record.Landing != nil {
		t.Fatalf("moved input did not reopen on its new base: %+v", record)
	}
}

func TestBatchLandingHeldRefusalNamesCause(t *testing.T) {
	_, store := landingBed(t)
	seams := greenLandSeams(&[]string{})
	seams.Held = func(_, _ string) error { return errors.New("held cause") }
	err := LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams)
	if err == nil || !strings.Contains(err.Error(), "BATCH_LAND_HELD_REFUSED") || !strings.Contains(err.Error(), "held cause") {
		t.Fatalf("held refusal=%v", err)
	}
}

func TestBatchLandingRequiresGreenProofActor(t *testing.T) {
	bed, _ := landingBed(t)
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "landing-owner"})
	store := NewStore(bed.root, nil)
	// landingBed already created this id in another root; this bed's root is
	// independent and the replacement store receives the actor-bound record.
	must(t, os.Remove(filepath.Join(bed.root, "artifacts", "agents", "landing-batches", testBatchID+".json")))
	must(t, store.Create(bed.record))
	if err := LandSeries(store, testBatchID, "other-owner", time.Unix(4, 0), greenLandSeams(&[]string{})); err == nil || !strings.Contains(err.Error(), "BATCH_LAND_DELEGATION_REFUSED") {
		t.Fatalf("foreign landing actor error=%v", err)
	}
	var events []string
	must(t, LandSeries(store, testBatchID, "landing-owner", time.Unix(4, 0), greenLandSeams(&events)))
	if !slices.Contains(events, "push") {
		t.Fatalf("owner landing events=%v", events)
	}
}

func TestBatchCommitRefusalIsAtomicAndKeepsJoinOrder(t *testing.T) {
	bed := assemblyFixture(t)
	bed.record.Units = append(bed.record.Units, Unit{GoalID: "goal-c", Chain: "chain-b", Claim: bed.record.Units[0].Claim, State: UnitJoined})
	bed.record.State, bed.record.PrefixTrees, bed.record.TipTree = StateLanding, []string{"prefix-a", "prefix-b", "prefix-c"}, "prefix-c"
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip"}
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
	bed.record.Receipts = map[string]PrefixReceipt{
		"goal-a": {GoalID: "goal-a", Tree: "prefix-a"},
		"goal-b": {GoalID: "goal-b", Tree: "prefix-b"},
	}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	var events []string
	seams := greenLandSeams(&events)
	seams.Commit = func(unit Unit, _ PrefixReceipt) (string, error) {
		events = append(events, "commit:"+unit.GoalID)
		if unit.GoalID == "goal-b" {
			return "", os.ErrPermission
		}
		return "commit-" + unit.GoalID, nil
	}
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), seams))
	record := load(t, store)
	if slices.Contains(events, "push") || !slices.Contains(events, "reset") || record.Units[1].State != UnitReturnPending || record.State != StateOpen || record.Units[0].State != UnitJoined || record.Units[2].State != UnitJoined {
		t.Fatalf("events=%v record=%+v", events, record)
	}
	if got := []string{record.Units[0].GoalID, record.Units[2].GoalID}; !slices.Equal(got, []string{"goal-a", "goal-c"}) {
		t.Fatalf("survivor order=%v", got)
	}
}

func TestBatchAfterPushRecoveryFinalizesEachTrailerOnce(t *testing.T) {
	_, store := landingBed(t)
	var events []string
	must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), greenLandSeams(&events)))
	finalized := map[string]int{}
	seams := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
		Finalize:     func(unit Unit, _ string) error { finalized[unit.GoalID]++; return nil },
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(5, 0), seams))
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(6, 0), seams))
	if finalized["goal-a"] != 1 || finalized["goal-b"] != 1 || load(t, store).State != StateLanded {
		t.Fatalf("finalized=%v record=%+v", finalized, load(t, store))
	}
}

func TestBatchRecoveryRefusalsNameCauses(t *testing.T) {
	t.Run("finalize", func(t *testing.T) {
		_, store := landingBed(t)
		must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), greenLandSeams(&[]string{})))
		err := RecoverPushedSeries(store, testBatchID, "owner", time.Unix(5, 0), RecoverySeams{
			OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
			Finalize:     func(Unit, string) error { return errors.New("finalize cause") },
		})
		if err == nil || !strings.Contains(err.Error(), "BATCH_P6_REFUSED") || !strings.Contains(err.Error(), "finalize cause") {
			t.Fatalf("finalize refusal=%v", err)
		}
	})
	t.Run("cleanup", func(t *testing.T) {
		_, store := landingBed(t)
		landingSeams := greenLandSeams(&[]string{})
		landingSeams.Cleanup = nil
		must(t, LandSeries(store, testBatchID, "owner", time.Unix(4, 0), landingSeams))
		err := RecoverPushedSeries(store, testBatchID, "owner", time.Unix(5, 0), RecoverySeams{
			OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
			Finalize:     func(Unit, string) error { return nil },
			Cleanup:      func() error { return errors.New("cleanup cause") },
		})
		if err == nil || !strings.Contains(err.Error(), "BATCH_P6_REFUSED") || !strings.Contains(err.Error(), "cleanup cause") {
			t.Fatalf("cleanup refusal=%v", err)
		}
	})
}

func TestApplyCertifiedPatchStagesTheTransportedBytes(t *testing.T) {
	bed := assemblyFixture(t)
	worktree := t.TempDir()
	baseCommit := strings.TrimSpace(runGitOutput(t, bed.root, "commit-tree", bed.base, "-m", "fixture base"))
	must(t, exec.Command("git", "-C", bed.root, "worktree", "add", "--detach", worktree, baseCommit).Run())
	t.Cleanup(func() { _ = exec.Command("git", "-C", bed.root, "worktree", "remove", "--force", worktree).Run() })
	must(t, ApplyCertifiedPatch(bed.root, worktree, "chain-a"))
	if got := strings.TrimSpace(runGitOutput(t, worktree, "diff", "--cached", "--name-only")); got != "a.go" {
		t.Fatalf("staged paths=%q", got)
	}
}

func TestCommitWithWrapperUsesTempRepoTokenAndExplicitIdentity(t *testing.T) {
	root := t.TempDir()
	must(t, os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755))
	must(t, os.MkdirAll(filepath.Join(root, "artifacts", "agents", "mains"), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "artifacts", "agents", "mains", "worktree-commit-token.json"), []byte("{}\n"), 0o644))
	wrapper := filepath.Join(root, "scripts", "agents", "commit.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail
test -f artifacts/agents/mains/worktree-commit-token.json
printf '%s\n' "$*" >wrapper.args
printf '%s <%s>|%s <%s>|%s\n' "$GIT_AUTHOR_NAME" "$GIT_AUTHOR_EMAIL" "$GIT_COMMITTER_NAME" "$GIT_COMMITTER_EMAIL" "$METASYSTEM_LANDED_BY" >wrapper.env
git commit -q "$@"
`
	must(t, os.WriteFile(wrapper, []byte(script), 0o755))
	must(t, exec.Command("git", "init", "-q", "-b", "main", root).Run())
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("one\n"), 0o644))
	must(t, exec.Command("git", "-C", root, "add", "file").Run())
	// The fixture wrapper consumes the boundary flags before delegating.
	script = strings.ReplaceAll(script, "git commit -q \"$@\"", `while (( $# )); do case "$1" in --chain|--goal|--test-receipt) shift 2;; *) break;; esac; done
git commit -q "$@"`)
	must(t, os.WriteFile(wrapper, []byte(script), 0o755))
	t.Setenv("GIT_AUTHOR_NAME", "Ambient Other")
	t.Setenv("GIT_AUTHOR_EMAIL", "ambient@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Ambient Other")
	t.Setenv("GIT_COMMITTER_EMAIL", "ambient@example.com")
	must(t, CommitWithWrapper(root, "chain-a", "goal-a", "receipt.json", "land goal a\n", "Wido", "wido@example.com", "m1l+landing-m1l"))
	if env := string(contents(t, filepath.Join(root, "wrapper.env"))); !strings.Contains(env, "Wido <wido@example.com>|Wido <wido@example.com>|m1l+landing-m1l") {
		t.Fatalf("wrapper env=%q", env)
	}
	if args := string(contents(t, filepath.Join(root, "wrapper.args"))); !strings.Contains(args, "--chain chain-a --goal goal-a --test-receipt receipt.json -F") {
		t.Fatalf("wrapper args=%q", args)
	}
}

func TestCommitWithRealWrapperWritesBatchTrailersAndExplicitIdentity(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"scripts/agents", "bin", "artifacts/agents/mains"} {
		must(t, os.MkdirAll(filepath.Join(root, dir), 0o755))
	}
	realWrapper, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "agents", "commit.sh"))
	must(t, err)
	must(t, os.WriteFile(filepath.Join(root, "scripts", "agents", "commit.sh"), realWrapper, 0o755))
	engine := `#!/usr/bin/env bash
set -euo pipefail
verb=${1:-}; noun=${2:-}
if [[ "$verb $noun" == "brain fence" ]]; then printf '{}\n'; exit 0; fi
if [[ "$verb $noun" == "lease require-holder" ]]; then printf '{"claimEpoch":1}\n'; exit 0; fi
if [[ "$verb $noun" == "lease run-held" ]]; then
  while (($#)) && [[ "$1" != -- ]]; do shift; done
  shift
  exec "$@"
fi
if [[ "$verb $noun" == "json get" ]]; then
  field=
  while (($#)); do if [[ "$1" == --field ]]; then field=$2; break; fi; shift; done
  case "$field" in
    claimEpoch) echo 1 ;;
    provenance) echo 'chain=chain-a change=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa' ;;
    verdictTrailer) echo 'pass bar=area' ;;
    code) echo reviewed-chain ;;
    mode) echo observe ;;
    goalRevision) echo 7 ;;
    refusal|detail) echo '' ;;
    *) echo '' ;;
  esac
  exit 0
fi
if [[ "$verb $noun" == "proc started-at" ]]; then echo 1; exit 0; fi
if [[ "$verb $noun" == "util token-hex" ]]; then echo 0123456789abcdef0123456789abcdef; exit 0; fi
if [[ "$verb $noun" == "lease commit-token" ]]; then
  token=
  while (($#)); do if [[ "$1" == --path ]]; then token=$2; break; fi; shift; done
  mkdir -p "$(dirname "$token")"; printf '{}\n' >"$token"; exit 0
fi
if [[ "$verb $noun" == "config conf-value" ]]; then echo testing.json; exit 0; fi
if [[ "$verb $noun" == "test verify" ]]; then exit 0; fi
if [[ "$verb $noun" == "behavior-surface select" ]]; then cat >/dev/null; exit 0; fi
if [[ "$verb $noun" == "landing observe" ]]; then
  printf '{"provenance":"chain=chain-a change=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","verdictTrailer":"pass bar=area","code":"reviewed-chain","mode":"observe","goalRevision":7}\n'
  exit 0
fi
if [[ "$verb $noun" == "gate weight-add" ]]; then cat >/dev/null; exit 0; fi
printf 'unexpected fake engine command: %s\n' "$*" >&2
exit 40
`
	must(t, os.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(engine), 0o755))
	build := `#!/usr/bin/env bash
set -euo pipefail
out=
while (($#)); do if [[ "$1" == --out ]]; then out=$2; shift 2; else shift; fi; done
cp "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)/bin/metasystem" "$out"
chmod +x "$out"
`
	must(t, os.WriteFile(filepath.Join(root, "scripts", "agents", "go-build.sh"), []byte(build), 0o755))
	must(t, os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("testing.contract=testing.json\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "testing.json"), []byte("{}\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, ".gitignore"), []byte("artifacts/agents/\n"), 0o644))
	must(t, os.WriteFile(filepath.Join(root, "source.txt"), []byte("base\n"), 0o644))
	must(t, exec.Command("git", "init", "-q", "-b", "main", root).Run())
	must(t, exec.Command("git", "-C", root, "config", "user.name", "Ambient Other").Run())
	must(t, exec.Command("git", "-C", root, "config", "user.email", "ambient@example.com").Run())
	must(t, exec.Command("git", "-C", root, "config", "metasystem.goal.machine", "m1l").Run())
	must(t, exec.Command("git", "-C", root, "add", ".").Run())
	must(t, exec.Command("git", "-C", root, "commit", "-qm", "base").Run())
	must(t, os.WriteFile(filepath.Join(root, "source.txt"), []byte("landed\n"), 0o644))
	must(t, exec.Command("git", "-C", root, "add", "source.txt").Run())
	t.Setenv("GIT_AUTHOR_NAME", "Ambient Other")
	t.Setenv("GIT_AUTHOR_EMAIL", "other@example.net")
	t.Setenv("GIT_COMMITTER_NAME", "Ambient Other")
	t.Setenv("GIT_COMMITTER_EMAIL", "other@example.net")
	t.Setenv("METASYSTEM_OWNER_LINEAGE", "landing-m1l")
	must(t, CommitWithWrapper(root, "chain-a", "goal-a", "receipt.json", "land goal a\n", "Wido Explicit", "wido@example.com", "m1l+landing-m1l"))
	message := runGitOutput(t, root, "show", "-s", "--format=%B", "HEAD")
	for _, trailer := range []string{
		"Machine: m1l+landing-m1l", "Goal-Item: goal-a", "Goal-Revision: 7",
		"Landing-Provenance: chain=chain-a change=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"Landing-Provenance-Verdict: pass bar=area", "Landed-By: m1l+landing-m1l",
	} {
		if !strings.Contains(message, trailer) {
			t.Fatalf("real commit message omitted %q:\n%s", trailer, message)
		}
	}
	identityLine := strings.TrimSpace(runGitOutput(t, root, "show", "-s", "--format=%an|%ae|%cn|%ce", "HEAD"))
	if identityLine != "Wido Explicit|wido@example.com|Wido Explicit|wido@example.com" {
		t.Fatalf("real commit used ambient identity: %q", identityLine)
	}
}

func TestLandingBranchLeasesEndpointAndDeletesCandidateAtomically(t *testing.T) {
	origin, root := filepath.Join(t.TempDir(), "origin.git"), t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run("init", "-q", "--bare", origin)
	run("init", "-q", "-b", "main", root)
	run("-C", root, "config", "user.name", "Fixture")
	run("-C", root, "config", "user.email", "fixture@example.com")
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("base\n"), 0o644))
	run("-C", root, "add", "file")
	run("-C", root, "commit", "-qm", "base")
	run("-C", root, "remote", "add", "origin", origin)
	run("-C", root, "push", "-q", "-u", "origin", "main")
	base := strings.TrimSpace(runGitOutput(t, root, "rev-parse", "HEAD"))
	must(t, PrepareLandingBranch(root, testBatchID, base))
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("tip\n"), 0o644))
	run("-C", root, "add", "file")
	tree := strings.TrimSpace(runGitOutput(t, root, "write-tree"))
	mid := strings.TrimSpace(runGitOutput(t, root, "commit-tree", tree, "-p", base, "-m", "candidate prefix"))
	tip := strings.TrimSpace(runGitOutput(t, root, "commit-tree", tree, "-p", mid, "-m", "candidate tip"))
	run("-C", root, "update-ref", "refs/heads/landing/"+testBatchID, tip)
	run("-C", root, "reset", "--hard", "-q", tip)
	must(t, PublishLandingBranch(root, testBatchID, "", tip))
	alien := strings.TrimSpace(runGitOutput(t, root, "commit-tree", tree, "-p", tip, "-m", "alien"))
	branchRef := "refs/heads/landing/" + testBatchID
	run("-C", root, "push", "-q", "--force", "origin", alien+":"+branchRef)
	if err := LandLandingBranch(root, testBatchID, base, tip); err == nil || !strings.Contains(err.Error(), "BATCH_LAND_PUSH_REFUSED") {
		t.Fatalf("moved landing branch error=%v", err)
	}
	if got := strings.TrimSpace(runGitOutput(t, origin, "rev-parse", "refs/heads/main")); got != base {
		t.Fatalf("atomic refusal moved endpoint to %s", got)
	}
	run("--git-dir", origin, "update-ref", branchRef, tip, alien)
	// A forward move to a commit in the candidate ancestry would be accepted
	// without the endpoint lease; the lease is the only refusal here.
	run("-C", root, "push", "-q", "--force", "origin", mid+":refs/heads/main")
	if err := LandLandingBranch(root, testBatchID, base, tip); err == nil || !strings.Contains(err.Error(), "BATCH_LAND_PUSH_REFUSED") {
		t.Fatalf("moved endpoint error=%v", err)
	}
	if err := exec.Command("git", "--git-dir", origin, "show-ref", "--verify", "--quiet", branchRef).Run(); err != nil {
		t.Fatalf("atomic endpoint refusal deleted candidate branch: %v", err)
	}
	run("--git-dir", origin, "update-ref", "refs/heads/main", base, mid)
	must(t, LandLandingBranch(root, testBatchID, base, tip))
	if got := strings.TrimSpace(runGitOutput(t, origin, "rev-parse", "refs/heads/main")); got != tip {
		t.Fatalf("endpoint=%s want %s", got, tip)
	}
	if err := exec.Command("git", "--git-dir", origin, "show-ref", "--verify", "--quiet", branchRef).Run(); err == nil {
		t.Fatalf("candidate branch %s survived atomic landing", branchRef)
	}
}

func TestAbandonLandingBranchIsIdempotent(t *testing.T) {
	origin, root := filepath.Join(t.TempDir(), "origin.git"), t.TempDir()
	run := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	run("init", "-q", "--bare", origin)
	run("init", "-q", "-b", "main", root)
	run("-C", root, "config", "user.name", "Fixture")
	run("-C", root, "config", "user.email", "fixture@example.com")
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("base\n"), 0o644))
	run("-C", root, "add", "file")
	run("-C", root, "commit", "-qm", "base")
	run("-C", root, "remote", "add", "origin", origin)
	run("-C", root, "push", "-q", "-u", "origin", "main")
	base := strings.TrimSpace(runGitOutput(t, root, "rev-parse", "HEAD"))
	must(t, PrepareLandingBranch(root, testBatchID, base))
	must(t, os.WriteFile(filepath.Join(root, "file"), []byte("tip\n"), 0o644))
	run("-C", root, "add", "file")
	run("-C", root, "commit", "-qm", "candidate")
	tip := strings.TrimSpace(runGitOutput(t, root, "rev-parse", "HEAD"))
	must(t, PublishLandingBranch(root, testBatchID, "", tip))
	must(t, AbandonLandingBranch(root, testBatchID, tip, base))
	must(t, AbandonLandingBranch(root, testBatchID, tip, base))
}

func runGitOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	must(t, err)
	return string(out)
}
