package usage

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestCallEvidenceSnapshotSerializesMaintenance(t *testing.T) {
	t.Run("snapshot excludes a concurrent writer", func(t *testing.T) {
		stateRoot := t.TempDir()
		transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
		firstAt := "2026-09-13T10:00:00Z"
		secondAt := "2026-09-13T11:00:00Z"
		writeCallRows(t, transcript, claudeAssistant("first", 10, 0, 0, false, firstAt))
		if _, err := LatestCall(stateRoot, "claude", "snapshot", ReadOptions{Capability: PerCall, Transcript: transcript}); err != nil {
			t.Fatal(err)
		}
		if err := RegisterSession(stateRoot, "claude", "snapshot", 101, 1001); err != nil {
			t.Fatal(err)
		}
		appendCallTestRow(t, transcript, claudeAssistant("second", 20, 0, 0, false, secondAt))

		snapshotPaused := make(chan struct{})
		releaseSnapshot := make(chan struct{})
		var pauseOnce sync.Once
		previousStep := callEvidenceSnapshotStep
		callEvidenceSnapshotStep = func(step string) {
			if step == "registrations" {
				pauseOnce.Do(func() {
					close(snapshotPaused)
					<-releaseSnapshot
				})
			}
		}
		t.Cleanup(func() { callEvidenceSnapshotStep = previousStep })

		type snapshotResult struct {
			evidence CallEvidence
			err      error
		}
		snapshotDone := make(chan snapshotResult, 1)
		go func() {
			evidence, err := ReadCallEvidence(stateRoot)
			snapshotDone <- snapshotResult{evidence: evidence, err: err}
		}()
		select {
		case <-snapshotPaused:
		case <-time.After(5 * time.Second):
			t.Fatal("evidence snapshot did not reach the registry barrier")
		}

		writerAttempted := make(chan struct{})
		var attemptOnce sync.Once
		previousOpen := callFileOpens
		callFileOpens = func(path string) {
			if path == callMaintenancePath(stateRoot) {
				attemptOnce.Do(func() { close(writerAttempted) })
			}
		}
		t.Cleanup(func() { callFileOpens = previousOpen })
		writerDone := make(chan error, 1)
		go func() {
			_, err := LatestCall(stateRoot, "claude", "snapshot", ReadOptions{Capability: PerCall, Transcript: transcript})
			writerDone <- err
		}()
		select {
		case <-writerAttempted:
		case <-time.After(5 * time.Second):
			close(releaseSnapshot)
			t.Fatal("writer did not attempt the maintenance lock")
		}
		select {
		case err := <-writerDone:
			close(releaseSnapshot)
			t.Fatalf("writer crossed the evidence snapshot: %v", err)
		default:
		}

		close(releaseSnapshot)
		var snapshot CallEvidence
		select {
		case result := <-snapshotDone:
			if result.err != nil {
				t.Fatal(result.err)
			}
			snapshot = result.evidence
		case <-time.After(5 * time.Second):
			t.Fatal("evidence snapshot did not release its maintenance lock")
		}
		select {
		case err := <-writerDone:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("writer did not continue after the evidence snapshot")
		}

		if snapshot.RetainedSince != (time.Time{}) ||
			!reflect.DeepEqual(snapshot.Sessions, []CallSession{{Runtime: "claude", Session: "snapshot"}}) ||
			len(snapshot.Registrations) != 1 || len(snapshot.Samples) != 1 || snapshot.Samples[0].InvocationID != "first" ||
			len(snapshot.Markers) != 0 {
			t.Fatalf("serialized snapshot = %#v", snapshot)
		}
		after, err := ReadCallEvidence(stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		if len(after.Samples) != 2 || after.Samples[1].InvocationID != "second" {
			t.Fatalf("evidence after writer = %#v", after.Samples)
		}
	})

	t.Run("public operations take maintenance before member locks", func(t *testing.T) {
		stateRoot := t.TempDir()
		transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
		writeCallRows(t, transcript, claudeAssistant("ordered", 10, 0, 0, false, "2026-09-13T10:00:00Z"))
		if _, err := LatestCall(stateRoot, "claude", "ordered", ReadOptions{Capability: PerCall, Transcript: transcript}); err != nil {
			t.Fatal(err)
		}
		if err := RegisterSession(stateRoot, "claude", "ordered", 201, 2001); err != nil {
			t.Fatal(err)
		}

		operations := []struct {
			name string
			run  func() error
		}{
			{"latest call", func() error {
				_, err := LatestCall(stateRoot, "claude", "ordered", ReadOptions{Capability: PerCall, Transcript: transcript})
				return err
			}},
			{"calls", func() error {
				_, _, err := Calls(stateRoot, "claude", "ordered", time.Time{})
				return err
			}},
			{"sessions", func() error {
				_, err := CallSessions(stateRoot)
				return err
			}},
			{"registrations", func() error {
				_, _, err := CallRegistrations(stateRoot)
				return err
			}},
			{"registration", func() error {
				return RegisterSession(stateRoot, "claude", "ordered", 201, 2001)
			}},
			{"evidence", func() error {
				_, err := ReadCallEvidence(stateRoot)
				return err
			}},
		}
		for _, operation := range operations {
			t.Run(operation.name, func(t *testing.T) {
				var opened []string
				previous := callFileOpens
				callFileOpens = func(path string) { opened = append(opened, path) }
				err := operation.run()
				callFileOpens = previous
				if err != nil {
					t.Fatal(err)
				}
				if len(opened) == 0 || opened[0] != callMaintenancePath(stateRoot) {
					t.Fatalf("open order = %v", opened)
				}
			})
		}
	})
}

func TestCallRetentionPreservesNonBlockingReads(t *testing.T) {
	stateRoot := t.TempDir()
	transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
	writeCallRows(t, transcript, claudeAssistant("busy", 10, 0, 0, false, "2026-09-13T10:00:00Z"))
	maintenance, err := lockCallMaintenance(stateRoot, true, false)
	if err != nil {
		t.Fatal(err)
	}
	locked := true
	release := func() {
		if locked {
			unlockCallFile(maintenance)
			locked = false
		}
	}
	defer release()

	assertStoreBusy := func(name string, operation func() error) {
		t.Helper()
		done := make(chan error, 1)
		go func() { done <- operation() }()
		select {
		case operationErr := <-done:
			var busy *CallStoreBusyError
			if !errors.As(operationErr, &busy) || busy.Path != callMaintenancePath(stateRoot) {
				t.Fatalf("%s error = %v", name, operationErr)
			}
		case <-time.After(time.Second):
			release()
			<-done
			t.Fatalf("%s waited on maintenance", name)
		}
	}
	assertStoreBusy("latest call", func() error {
		_, err := LatestCall(stateRoot, "claude", "busy", ReadOptions{
			Capability: PerCall, Transcript: transcript, NonBlocking: true,
		})
		return err
	})
	assertStoreBusy("registration", func() error {
		return RegisterSessionNonBlocking(stateRoot, "claude", "busy", 301, 3001)
	})

	for _, path := range []string{
		CursorPath(stateRoot, "claude", "busy") + ".lock",
		filepath.Join(stateRoot, "artifacts", "agents", "context", "sessions.jsonl.lock"),
	} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("nonblocking operation reached inner lock %s: %v", path, err)
		}
	}
}

func TestCallEvidenceSnapshotBlocksPruneAtEveryBoundary(t *testing.T) {
	for _, boundary := range []struct {
		name       string
		step       string
		occurrence int
	}{
		{"registry snapshot", "registrations", 1},
		{"session discovery", "sessions", 1},
		{"first committed rows", "rows", 1},
		{"second committed rows", "rows", 2},
	} {
		t.Run(boundary.name, func(t *testing.T) {
			root := t.TempDir()
			before := retirementTestCutoff()
			old := before.Add(-4 * time.Hour)
			for _, session := range []string{"first", "second"} {
				seedRetirementCall(t, root, session, old)
				setRetirementPairTimes(t, root, "claude", session, old, old)
			}

			paused := make(chan struct{})
			release := make(chan struct{})
			seen := 0
			previousStep := callEvidenceSnapshotStep
			callEvidenceSnapshotStep = func(step string) {
				if step == boundary.step {
					seen++
					if seen == boundary.occurrence {
						close(paused)
						<-release
					}
				}
			}
			t.Cleanup(func() { callEvidenceSnapshotStep = previousStep })
			type evidenceResult struct {
				evidence CallEvidence
				err      error
			}
			evidenceDone := make(chan evidenceResult, 1)
			go func() {
				evidence, err := ReadCallEvidence(root)
				evidenceDone <- evidenceResult{evidence: evidence, err: err}
			}()
			select {
			case <-paused:
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatalf("evidence read did not reach %s occurrence %d", boundary.step, boundary.occurrence)
			}

			pruneAttempted := make(chan struct{})
			var attemptOnce sync.Once
			previousOpen := callFileOpens
			callFileOpens = func(path string) {
				if path == callMaintenancePath(root) {
					attemptOnce.Do(func() { close(pruneAttempted) })
				}
			}
			t.Cleanup(func() { callFileOpens = previousOpen })
			type pruneResult struct {
				removed int
				err     error
			}
			pruneDone := make(chan pruneResult, 1)
			go func() {
				removed, err := PruneCallSessions(root, before)
				pruneDone <- pruneResult{removed: removed, err: err}
			}()
			select {
			case <-pruneAttempted:
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("prune did not attempt the maintenance lock")
			}
			select {
			case result := <-pruneDone:
				close(release)
				t.Fatalf("prune crossed %s: %+v", boundary.name, result)
			default:
			}
			close(release)
			evidence := <-evidenceDone
			if evidence.err != nil || len(evidence.evidence.Samples) != 2 {
				t.Fatalf("evidence at %s = %+v err=%v", boundary.name, evidence.evidence, evidence.err)
			}
			pruned := <-pruneDone
			if pruned.err != nil || pruned.removed != 2 {
				t.Fatalf("prune after %s removed=%d err=%v", boundary.name, pruned.removed, pruned.err)
			}
			callEvidenceSnapshotStep = previousStep
			callFileOpens = previousOpen
		})
	}
}
