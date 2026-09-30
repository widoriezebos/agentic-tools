package diskstore

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ignoreLocalConfig makes the bed's repository ignore what the engine
// places in a worktree: the runtime's local configuration and the
// installation's artifacts.
func (b *linkedBed) ignoreLocalConfig() {
	b.t.Helper()
	if err := os.WriteFile(filepath.Join(b.repo, ".gitignore"), []byte("metasystem/artifacts/\n.claude/settings.local.json\n"), 0o600); err != nil {
		b.t.Fatal(err)
	}
	b.must(b.repo, "add", ".gitignore")
	b.must(b.repo, "commit", "-q", "-m", "ignore local config")
}

// placeSettings copies the local configuration into the worktree the way
// the isolation owner does, and records it as the engine's.
func (b *linkedBed) placeSettings(record Record) string {
	b.t.Helper()
	settings := filepath.Join(record.Path, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o700); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte("{}\n"), 0o600); err != nil {
		b.t.Fatal(err)
	}
	if err := b.registry.RecordEngineContent(record.ID, record.Path, []string{".claude/settings.local.json"}, nil); err != nil {
		b.t.Fatal(err)
	}
	return settings
}

// Round D3 F-1 (P1): a goal worktree holding only the local configuration
// the engine copied in is released by goal done's registered sweep; the
// same file edited afterwards is someone's work and keeps it; an ignored
// file the engine never placed keeps it.
func TestGoalDoneReleasesAWorktreeHoldingOnlyTheEnginesUnchangedFiles(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		alter func(t *testing.T, record Record, settings string)
		done  bool
	}{
		{name: "only the engine's copy", done: true},
		{name: "the copy edited", alter: func(t *testing.T, _ Record, settings string) {
			if err := os.WriteFile(settings, []byte(`{"mine":true}`+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "a foreign ignored file", alter: func(t *testing.T, record Record, _ string) {
			if err := os.MkdirAll(filepath.Join(record.Path, "metasystem", "artifacts"), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(record.Path, "metasystem", "artifacts", "notes.txt"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			bed := newLinkedBed(t)
			bed.ignoreLocalConfig()
			record := bed.goalWorktree("g")
			settings := bed.placeSettings(record)
			if test.alter != nil {
				test.alter(t, record, settings)
			}
			ran := 0
			outcome, err := ReleaseLinkedWorktrees(context.Background(), bed.registry, []string{record.ID}, LinkedRelease{GitRoot: bed.repo, Git: realWorkspaceGit,
				Census: &UseCensus{Taken: true}, Now: testNow, By: "goal done",
				Remove: func(ctx context.Context) error { ran++; return bed.goalSweep(ctx, "g") }})
			if err != nil {
				t.Fatal(err)
			}
			if outcome.Done != test.done || (ran == 1) != test.done {
				t.Fatalf("outcome = %+v, removals %d, want released %v", outcome, ran, test.done)
			}
			if !test.done && !outcome.Kept {
				t.Fatalf("a worktree with foreign content is kept: %+v", outcome)
			}
		})
	}
}

// Round D3 F-1: a finished session's worktree whose only ignored content is
// the engine's (its copied configuration and its main announcements) is
// released; an unknown name in the announcements directory keeps it.
func TestAFinishedSessionsWorktreeHoldingOnlyEngineContentIsReleased(t *testing.T) {
	t.Parallel()
	for _, foreign := range []bool{false, true} {
		bed := newLinkedBed(t)
		bed.ignoreLocalConfig()
		record := bed.sessionWorktree("side")
		mains := filepath.Join("metasystem", "artifacts", "agents", "mains")
		if err := bed.registry.RecordEngineContent(record.ID, record.Path, nil, []EngineDir{{Path: mains, Pattern: "*.json", Format: FormatJSONObject}}); err != nil {
			t.Fatal(err)
		}
		bed.placeSettings(record)
		if err := os.MkdirAll(filepath.Join(record.Path, mains), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(record.Path, mains, "main-1.json"), []byte(`{"sessionId":"s1","pid":1}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if foreign {
			if err := os.WriteFile(filepath.Join(record.Path, mains, "scratch.txt"), []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		proof := SessionWorktreeProof{GitRoot: bed.repo, Git: realWorkspaceGit, Grace: 24 * time.Hour,
			Main:          func(string) (MainLiveness, string) { return MainDead, "every announced main is dead" },
			BootstrapDead: func(string) (bool, bool) { return true, true }}
		report := bed.sessionSweep(proof, censusOf(), testNow.Add(time.Hour))
		if released := len(report.Actions) == 1; released == foreign {
			t.Fatalf("foreign=%v: report %+v", foreign, report)
		}
		if foreign && !keptLine(report, record.Path, "scratch.txt") {
			t.Fatalf("the unknown name is named: %+v", report)
		}
	}
}

// An unreadable engine-content record keeps the worktree (fail-closed rule
// 1).
func TestAnUnreadableEngineContentRecordKeepsTheWorktree(t *testing.T) {
	t.Parallel()
	bed := newLinkedBed(t)
	bed.ignoreLocalConfig()
	record := bed.goalWorktree("g")
	bed.placeSettings(record)
	if err := os.WriteFile(bed.registry.EngineContentPath(record.ID), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	outcome, err := ReleaseLinkedWorktrees(context.Background(), bed.registry, []string{record.ID}, LinkedRelease{GitRoot: bed.repo, Git: realWorkspaceGit,
		Census: &UseCensus{Taken: true}, Now: testNow, By: "goal done", Remove: func(ctx context.Context) error { return bed.goalSweep(ctx, "g") }})
	if err != nil || outcome.Done || !strings.Contains(outcome.Reason, "unreadable") {
		t.Fatalf("outcome = %+v, %v", outcome, err)
	}
}
