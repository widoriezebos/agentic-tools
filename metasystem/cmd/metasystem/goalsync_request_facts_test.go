package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

var syncRequestTestNow = time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)

type syncRequestFacts struct {
	t                                                              *testing.T
	root                                                           string
	reader                                                         *goalSyncEnrollmentReader
	events                                                         []string
	topCalls, ledgerCalls, guardCalls, endpointCalls, machineCalls int
	lineageCalls, clockCalls, humanCalls, terminalCalls            int
}

func newSyncRequestFacts(t *testing.T) *syncRequestFacts {
	t.Helper()
	root := t.TempDir()
	for path, data := range map[string][]byte{
		"metasystem.conf":                    []byte("metasystem.runtimes=fake\nmetasystem.governance.correlation-policy=A\n"),
		"scripts/agents/pre-commit-guard.sh": []byte("#!/bin/sh\nexit 0\n"),
		"plans/goals/backlog.md": goal.RenderRoot(&goal.RootRecord{
			Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: goal.SyncLocal, Revision: 1,
		}),
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if path == "scripts/agents/pre-commit-guard.sh" {
			mode = 0o755
		}
		if err := os.WriteFile(full, data, mode); err != nil {
			t.Fatal(err)
		}
	}
	return &syncRequestFacts{t: t, root: root}
}

func (f *syncRequestFacts) checkRoot(root string) {
	f.t.Helper()
	if root != f.root {
		f.t.Fatalf("request root = %q, want %q", root, f.root)
	}
}

func (f *syncRequestFacts) dependencies() syncRequestDependencies {
	return syncRequestDependencies{
		authorityFacts: goalAuthorityReadFacts{
			repositoryTop: func(root string) (string, error) {
				f.checkRoot(root)
				f.topCalls++
				f.events = append(f.events, "repository top")
				return f.root, nil
			},
			ledgerIdentity: func(root string) string {
				f.checkRoot(root)
				f.ledgerCalls++
				f.events = append(f.events, "ledger identity")
				data, err := os.ReadFile(filepath.Join(root, "plans", "goals", "backlog.md"))
				if err != nil {
					f.t.Fatal(err)
				}
				record, problems := goal.ParseRoot(data)
				if record == nil || len(problems) != 0 {
					f.t.Fatalf("physical ledger root: record=%+v problems=%v", record, problems)
				}
				return record.Identity
			},
		},
		ensureGuard: func(root string) error {
			f.checkRoot(root)
			f.guardCalls++
			f.events = append(f.events, "guard")
			return nil
		},
		endpoint: func(root string) (goal.Endpoint, error) {
			f.checkRoot(root)
			f.endpointCalls++
			f.events = append(f.events, "endpoint")
			return goal.Endpoint{Root: root, Remote: "local", Branch: "refs/heads/main", Repository: syncRequestUnexpectedRepository{t: f.t}}, nil
		},
		machine: func(root string) (string, error) {
			f.checkRoot(root)
			f.machineCalls++
			f.events = append(f.events, "machine")
			return "mac-cli", nil
		},
		ownerLineage: func() string { f.lineageCalls++; f.events = append(f.events, "lineage"); return "" },
		proveHuman: func(root string, _ int64, _ humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
			f.checkRoot(root)
			f.humanCalls++
			f.events = append(f.events, "human proof")
			if f.reader == nil {
				f.t.Fatal("unexpected enrolled proof call")
			}
			return humanauthority.Prove(root, f.reader.exact.Pid, *f.reader, now)
		},
		proveTerminal: func(root string, _ int64, _ humanauthority.Reader, now time.Time) (humanauthority.Proof, error) {
			f.checkRoot(root)
			f.terminalCalls++
			f.events = append(f.events, "terminal proof")
			if f.reader == nil {
				f.t.Fatal("unexpected terminal proof call")
			}
			return humanauthority.ProveTerminal(root, f.reader.exact.Pid, *f.reader, now)
		},
	}
}

func (f *syncRequestFacts) commandNow(root string) (time.Time, error) {
	f.checkRoot(root)
	f.clockCalls++
	f.events = append(f.events, "clock")
	return syncRequestTestNow, nil
}

func (f *syncRequestFacts) expect(classifications, calls, lineage, clock, human, terminal int, sequence ...string) {
	f.t.Helper()
	if f.topCalls != classifications || f.ledgerCalls != classifications || f.guardCalls != calls || f.endpointCalls != calls || f.machineCalls != calls ||
		f.lineageCalls != lineage || f.clockCalls != clock || f.humanCalls != human || f.terminalCalls != terminal {
		f.t.Fatalf("request fact calls: top=%d ledger=%d guard=%d endpoint=%d machine=%d lineage=%d clock=%d human=%d terminal=%d; want classifications=%d calls=%d lineage=%d clock=%d human=%d terminal=%d",
			f.topCalls, f.ledgerCalls, f.guardCalls, f.endpointCalls, f.machineCalls, f.lineageCalls, f.clockCalls, f.humanCalls, f.terminalCalls,
			classifications, calls, lineage, clock, human, terminal)
	}
	if !reflect.DeepEqual(f.events, sequence) {
		f.t.Fatalf("request fact sequence = %v, want %v", f.events, sequence)
	}
}

type syncRequestUnexpectedRepository struct {
	t *testing.T
}

func (r syncRequestUnexpectedRepository) unexpected(name string) {
	r.t.Helper()
	r.t.Fatalf("request constructor called repository %s", name)
}
func (r syncRequestUnexpectedRepository) Capture(string) (string, error) {
	r.unexpected("Capture")
	return "", nil
}
func (r syncRequestUnexpectedRepository) Accepted() (string, bool, error) {
	r.unexpected("Accepted")
	return "", false, nil
}
func (r syncRequestUnexpectedRepository) Files(string, ...string) (map[string][]byte, error) {
	r.unexpected("Files")
	return nil, nil
}
func (r syncRequestUnexpectedRepository) Build(string, string, []goal.Change, string) (string, error) {
	r.unexpected("Build")
	return "", nil
}
func (r syncRequestUnexpectedRepository) Publish(string, string) (goal.CASOutcome, error) {
	r.unexpected("Publish")
	return "", nil
}
func (r syncRequestUnexpectedRepository) AcceptedCAS(string, string) error {
	r.unexpected("AcceptedCAS")
	return nil
}
func (r syncRequestUnexpectedRepository) IsAncestor(string, string) (bool, error) {
	r.unexpected("IsAncestor")
	return false, nil
}
func (r syncRequestUnexpectedRepository) TrailerPresent(string, string) (bool, error) {
	r.unexpected("TrailerPresent")
	return false, nil
}
func (r syncRequestUnexpectedRepository) CommitWithTrailer(string, string, string) (string, error) {
	r.unexpected("CommitWithTrailer")
	return "", nil
}
func (r syncRequestUnexpectedRepository) CommitTime(string) (time.Time, error) {
	r.unexpected("CommitTime")
	return time.Time{}, nil
}
func (r syncRequestUnexpectedRepository) Release(string) error { r.unexpected("Release"); return nil }

var _ goal.Repository = syncRequestUnexpectedRepository{}
