package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func writeJob(t *testing.T, f *pendingWaitVerdictFixture, status string, round int64, startedAt string) {
	writePendingWaitJSON(t, filepath.Join(f.root, "artifacts", "agents", "jobs", f.row.TargetID+".json"), map[string]any{"jobId": f.row.TargetID, "operationId": f.row.Target.OperationID, "round": round, "startedAt": startedAt, "status": status, "goalId": pendingWaitGoalID})
}

func TestRegisteredWaitDropReasons(t *testing.T) {
	const unreadable = "plans/broken.md: permission denied"
	annPath := func(f *pendingWaitVerdictFixture) string {
		return filepath.Join(f.root, "artifacts", "agents", "mains", pendingWaitSession+"-41.json")
	}
	jobPath := func(f *pendingWaitVerdictFixture) string {
		return filepath.Join(f.root, "artifacts", "agents", "jobs", f.row.TargetID+".json")
	}
	tests := []struct {
		name, branch, want, kind string
		mutate                   func(*testing.T, *pendingWaitVerdictFixture)
		direct                   bool
	}{
		{name: "control", mutate: func(*testing.T, *pendingWaitVerdictFixture) {}},
		{name: "no-own-row", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			f.row.OwnerDigest = metarun.OwnerDigest("other")
			f.writeRow(t)
			writePendingWaitJSON(t, sessionStopLeasePath(f.root), sessionStopLease{HolderMainId: "other"})
		}},
		{name: "owner-lease", branch: "owner-lease", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			writePendingWaitJSON(t, sessionStopLeasePath(f.root), sessionStopLease{HolderMainId: "other"})
		}},
		{name: "owner-announcement", branch: "owner-announcement", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			writePendingWaitJSON(t, annPath(f), map[string]any{"bad": true})
		}},
		{name: "owner-announcement-missing", branch: "owner-announcement-missing", mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) { _ = os.Remove(annPath(f)) }},
		{name: "boot-clock", branch: "boot-clock", mutate: func(_ *testing.T, _ *pendingWaitVerdictFixture) {
			turnVerdictBootClock = func() (string, time.Duration, error) { return "", 0, os.ErrInvalid }
		}},
		{name: "row-unreadable", branch: "row-unreadable", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			old := metarun.WaiterPath(f.root, f.row.Kind, f.row.TargetID, f.row.OwnerDigest)
			_ = os.Rename(old, filepath.Join(metarun.WaitersDir(f.root), "wrong-"+f.row.OwnerDigest+".json"))
		}},
		{name: "row-coordinates", branch: "row-coordinates", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.Session = "other"; f.writeRow(t) }},
		{name: "row-process", branch: "row-process", mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) { delete(f.store.Prober.(idleFixtureProber), 42) }},
		{name: "row-wall-clock", branch: "row-wall-clock", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.LastObservedAt = "bad"; f.writeRow(t) }},
		{name: "row-boot-clock", branch: "row-boot-clock", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.DeadlineBootID = "other"; f.writeRow(t) }},
		{name: "job-record", branch: "job-record", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.Target.Round++; f.writeRow(t) }},
		{name: "not-claimed", branch: "not-claimed", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			writePendingWaitJSON(t, jobPath(f), map[string]any{"jobId": f.row.TargetID, "operationId": f.row.Target.OperationID, "round": f.row.Target.Round, "startedAt": f.row.Target.StartedAt, "status": "running", "goalId": "other"})
		}},
		{name: "source", branch: "source", kind: "run", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.Target.LaunchNonce = "other"; f.writeRow(t) }},
		{name: "row-nonregular", branch: "row-unreadable", want: "check=lstat-or-regular", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			_ = os.Remove(metarun.WaiterPath(f.root, f.row.Kind, f.row.TargetID, f.row.OwnerDigest))
			_ = os.Mkdir(filepath.Join(metarun.WaitersDir(f.root), "x-"+f.row.OwnerDigest+".json"), 0o755)
		}},
		{name: "row-read", branch: "row-unreadable", want: "check=read", mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) {
			_ = os.Chmod(metarun.WaiterPath(f.root, f.row.Kind, f.row.TargetID, f.row.OwnerDigest), 0)
		}},
		{name: "announcement-read", branch: "owner-announcement", want: "file=" + pendingWaitSession + "-41.json", direct: true, mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) { _ = os.Chmod(annPath(f), 0) }},
		{name: "announcement-identity", branch: "owner-announcement", want: "lineage=" + pendingWaitLineage, mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) {
			_ = os.Link(annPath(f), filepath.Join(filepath.Dir(annPath(f)), "zz-42.json"))
		}},
		{name: "goal-selector", branch: "row-coordinates", want: "goalId=", kind: "human-act", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.GoalID = ""; f.writeRow(t) }},
		{name: "non-goal-id", branch: "row-coordinates", want: "goalId=other", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.GoalID = "other"; f.writeRow(t) }},
		{name: "attempt-source", branch: "source", want: "check=attempt-record", kind: "attempt", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { f.row.Target.ProofDigest = "other"; f.writeRow(t) }},
		{name: "goal-not-claimed", branch: "not-claimed", want: "goalId=other", kind: "human-act", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			f.row.TargetID, f.row.Selector.TargetID, f.row.GoalID, f.row.Selector.GoalID = "other", "other", "other", "other"
			f.writeRow(t)
		}},
		{name: "job-read", branch: "job-record", want: "err=open", mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) { _ = os.Remove(jobPath(f)) }},
		{name: "job-decode", branch: "job-record", want: "err=unexpected end", direct: true, mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) { _ = os.WriteFile(jobPath(f), []byte("{"), 0o644) }},
		{name: "job-pending-setup", branch: "job-record", want: "recordStatus=pending-setup", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { writeJob(t, f, "pending-setup", 0, "") }},
		{name: "job-status", branch: "job-record", want: "recordStatus=done", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			writeJob(t, f, "done", 1, f.row.Target.StartedAt)
		}},
		{name: "job-round", branch: "job-record", want: "recordRound=0", mutate: func(t *testing.T, f *pendingWaitVerdictFixture) { writeJob(t, f, "running", 0, "") }},
		{name: "work-unreadable", branch: "work-unreadable", want: unreadable, mutate: func(_ *testing.T, f *pendingWaitVerdictFixture) { f.scan.Unreadable = []string{unreadable} }},
		{name: "work-unreadable-no-own", want: unreadable, mutate: func(t *testing.T, f *pendingWaitVerdictFixture) {
			f.row.OwnerDigest = metarun.OwnerDigest("other")
			f.writeRow(t)
			f.scan.Unreadable = []string{unreadable}
		}},
		{name: "owner-empty-session", branch: "owner-lease", want: "sessionId=", direct: true, mutate: func(*testing.T, *pendingWaitVerdictFixture) {}},
		{name: "owner-glob", branch: "owner-announcement", want: "err=syntax error in pattern", direct: true, mutate: func(*testing.T, *pendingWaitVerdictFixture) {}},
		{name: "landing-not-claimed", branch: "not-claimed", want: "claimedIds=[]", kind: "landing", direct: true, mutate: func(*testing.T, *pendingWaitVerdictFixture) {}},
		{name: "goal-budget", branch: "source", want: "check=budget", kind: "human-act", direct: true, mutate: func(*testing.T, *pendingWaitVerdictFixture) {}},
		{name: "goal-observation", branch: "source", want: "check=observation", kind: "human-act", direct: true, mutate: func(*testing.T, *pendingWaitVerdictFixture) {}},
		{name: "unknown-kind", branch: "source", want: "check=known-kind", kind: "human-act", direct: true, mutate: func(*testing.T, *pendingWaitVerdictFixture) {}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kind := map[bool]string{false: tc.kind, true: "job"}[tc.kind == ""]
			fixture := newPendingWaitVerdictFixture(t, kind, false)
			tc.mutate(t, fixture)
			if tc.direct {
				if reason := directWaitDropSite(t, fixture, tc.name); !strings.HasPrefix(reason, tc.branch+": ") || !strings.Contains(reason, tc.want) {
					t.Fatalf("reason=%q want branch=%q value=%q", reason, tc.branch, tc.want)
				}
				return
			}
			verdict := fixture.verdict(t, fixture.scan)
			diagnostics := strings.Join(verdict.Diagnostics, "\n")
			if tc.branch == "" {
				if strings.Contains(diagnostics, " not credited at ") || (tc.name == "control") != strings.Contains(verdict.Display, "WAITING: registered wait") {
					t.Fatalf("%s: diagnostics=%q display=%q", tc.name, diagnostics, verdict.Display)
				}
				return
			}
			artifact, _ := os.ReadFile(turnVerdictArtifactPath(fixture.root, pendingWaitSession))
			if strings.Count(diagnostics, " not credited at ") != 1 || !strings.Contains(diagnostics, " not credited at "+tc.branch+": ") || !strings.Contains(string(artifact), verdict.Diagnostics[0]) || strings.Contains(verdict.Display, "WAITING: registered wait") || strings.HasPrefix(tc.branch, "row-") && !strings.Contains(diagnostics, "registered wait file ") && (!strings.Contains(diagnostics, fixture.row.WaitID) || !strings.Contains(diagnostics, "=")) || !strings.Contains(diagnostics, tc.want) {
				t.Fatalf("%s: diagnostics=%q display=%q artifact=%q", tc.branch, diagnostics, verdict.Display, artifact)
			}
		})
	}
}

func directWaitDropSite(t *testing.T, f *pendingWaitVerdictFixture, site string) (reason string) {
	if site == "owner-empty-session" {
		f.store.registeredWaitOwner("", pendingWaitMainID, &reason)
	} else if site == "owner-glob" {
		f.store.Root = filepath.Join(t.TempDir(), "[")
		_ = os.Symlink(f.root, f.store.Root)
		f.store.registeredWaitOwner(pendingWaitSession, pendingWaitMainID, &reason)
	} else if site == "announcement-read" {
		f.store.registeredWaitOwner(pendingWaitSession, pendingWaitMainID, &reason)
	} else if site == "job-decode" {
		f.store.pendingWaitJob(f.row, &reason)
	} else {
		row := f.row
		row.RemainingNanos = map[bool]int64{false: row.RemainingNanos, true: 0}[site == "goal-budget"]
		row.Target.ProofDigest = map[bool]string{false: row.Target.ProofDigest, true: "other"}[site == "goal-observation"]
		row.Kind = map[bool]string{false: row.Kind, true: "unknown"}[site == "unknown-kind"]
		f.store.registeredWaitSource(ClaimableBudgetedWork{Claimed: []string{pendingWaitGoalID}}, map[string]bool{pendingWaitGoalID: true}, row, f.bootElapsed, &reason)
	}
	return
}
