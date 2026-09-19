package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPruneCallSessionsUsesTheNewerMemberAge(t *testing.T) {
	root := t.TempDir()
	before := retirementTestCutoff()
	old := before.Add(-4 * time.Hour)
	fresh := before.Add(time.Hour)

	seedRetirementCall(t, root, "old", old)
	setRetirementPairTimes(t, root, "claude", "old", old, old)
	seedRetirementCall(t, root, "fresh-cursor", old)
	setRetirementPairTimes(t, root, "claude", "fresh-cursor", fresh, old)
	seedRetirementCall(t, root, "fresh-samples", old)
	setRetirementPairTimes(t, root, "claude", "fresh-samples", old, fresh)
	seedRetirementCall(t, root, "equal", old)
	setRetirementPairTimes(t, root, "claude", "equal", old, before)
	seedRetirementCall(t, root, "fresh-row", fresh)
	setRetirementPairTimes(t, root, "claude", "fresh-row", old, old)
	seedRetirementCallWithRows(t, root, "fresh-marker", old,
		claudeAssistant("fresh-marker", 10, 0, 0, false, old.Format(time.RFC3339Nano)),
		fmt.Sprintf(`{"type":"system","subtype":"compact_boundary","timestamp":%q,"compactMetadata":{"trigger":"auto","preTokens":10}}`, fresh.Format(time.RFC3339Nano)))
	setRetirementPairTimes(t, root, "claude", "fresh-marker", old, old)
	seedRetirementCall(t, root, "fresh-registration", old)
	setRetirementPairTimes(t, root, "claude", "fresh-registration", old, old)
	appendRetirementRegistration(t, root, CallRegistration{
		Runtime: "claude", Session: "fresh-registration", PID: 999, PIDStartedAt: 9999, FirstSeen: fresh,
	})
	orphan := SamplesPath(root, "orphan", "empty")
	if err := os.MkdirAll(filepath.Dir(orphan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(orphan, old, old); err != nil {
		t.Fatal(err)
	}

	registryPath := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl")
	registryBefore, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	removed, err := PruneCallSessions(root, before)
	if err != nil || removed != 2 {
		t.Fatalf("prune removed=%d err=%v", removed, err)
	}
	for _, session := range []string{"fresh-cursor", "fresh-samples", "equal", "fresh-row", "fresh-marker", "fresh-registration"} {
		assertRetirementPairPresent(t, root, "claude", session)
	}
	assertRetirementPairAbsent(t, root, "claude", "old")
	if _, err := os.Lstat(orphan); !os.IsNotExist(err) {
		t.Fatalf("empty samples orphan survived: %v", err)
	}
	registryAfter, err := os.ReadFile(registryPath)
	if err != nil || !reflect.DeepEqual(registryAfter, registryBefore) {
		t.Fatalf("registry changed: err=%v", err)
	}
	retention, err := readCallRetention(root)
	if err != nil || !retention.RetainedSince.Equal(callRetentionBoundary(before)) {
		t.Fatalf("retention=%+v err=%v", retention, err)
	}
	if _, err := os.Stat(CursorPath(root, "claude", "old") + ".lock"); err != nil {
		t.Fatalf("stable cursor lock was removed: %v", err)
	}

	t.Run("removed counts an orphan whose cleanup sync fails", func(t *testing.T) {
		orphanRoot := t.TempDir()
		path := SamplesPath(orphanRoot, "orphan", "sync-failure")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
		originalSync := syncCallStoreDirectory
		syncCallStoreDirectory = func(string) error { return errors.New("injected orphan sync failure") }
		t.Cleanup(func() { syncCallStoreDirectory = originalSync })
		removed, err := PruneCallSessions(orphanRoot, before)
		if err == nil || removed != 1 {
			t.Fatalf("orphan cleanup removed=%d err=%v", removed, err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("orphan still exists after counted unlink: %v", err)
		}
	})
}

func TestPruneCallSessionsRefusesFutureCutoff(t *testing.T) {
	now := time.Now().UTC()
	originalNow := callRetentionNow
	callRetentionNow = func() time.Time { return now }
	t.Cleanup(func() { callRetentionNow = originalNow })
	for _, test := range []struct {
		name   string
		cutoff time.Time
	}{
		{"exactly now", now},
		{"one nanosecond ahead", now.Add(time.Nanosecond)},
		{"one nanosecond behind", now.Add(-time.Nanosecond)},
		{"just inside retention window", now.Add(-callRetentionWindow + time.Nanosecond)},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			recordedAt := now.Add(-callRetentionWindow - 2*time.Hour)
			seedRetirementCall(t, root, "guarded-cutoff", recordedAt)
			setRetirementPairTimes(t, root, "claude", "guarded-cutoff", recordedAt, recordedAt)

			removed, err := PruneCallSessions(root, test.cutoff)
			if err == nil || removed != 0 || !strings.Contains(err.Error(), "window=14d") ||
				!strings.Contains(err.Error(), "cutoff="+test.cutoff.Format(time.RFC3339Nano)) {
				t.Fatalf("guarded cutoff=%s removed=%d err=%v", test.cutoff.Format(time.RFC3339Nano), removed, err)
			}
			assertRetirementPairPresent(t, root, "claude", "guarded-cutoff")
			retention, err := readCallRetention(root)
			if err != nil || !retention.RetainedSince.IsZero() {
				t.Fatalf("guarded cutoff published retention=%+v err=%v", retention, err)
			}
		})
	}

	t.Run("outside retention window", func(t *testing.T) {
		root := t.TempDir()
		cutoff := now.Add(-callRetentionWindow - 24*time.Hour)
		recordedAt := cutoff.Add(-time.Hour)
		seedRetirementCall(t, root, "retirable-cutoff", recordedAt)
		setRetirementPairTimes(t, root, "claude", "retirable-cutoff", recordedAt, recordedAt)

		removed, err := PruneCallSessions(root, cutoff)
		if err != nil || removed != 1 {
			t.Fatalf("retirable cutoff removed=%d err=%v", removed, err)
		}
		assertRetirementPairAbsent(t, root, "claude", "retirable-cutoff")
		retention, err := readCallRetention(root)
		if err != nil || !retention.RetainedSince.Equal(callRetentionBoundary(cutoff)) {
			t.Fatalf("retirable cutoff published retention=%+v err=%v", retention, err)
		}
	})
}

func TestPruneCallSessionsDoesNotAdvanceRetentionForEmptyOrphan(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	before := now.Add(-callRetentionWindow - 24*time.Hour)
	liveAt := now.Add(-time.Hour)
	seedRetirementCall(t, root, "live", liveAt)
	setRetirementPairTimes(t, root, "claude", "live", liveAt, liveAt)

	orphan := SamplesPath(root, "orphan", "empty")
	if err := os.MkdirAll(filepath.Dir(orphan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := before.Add(-time.Hour)
	if err := os.Chtimes(orphan, old, old); err != nil {
		t.Fatal(err)
	}

	removed, err := PruneCallSessions(root, before)
	if err != nil || removed != 1 {
		t.Fatalf("orphan prune removed=%d err=%v", removed, err)
	}
	if _, err := os.Lstat(orphan); !os.IsNotExist(err) {
		t.Fatalf("empty samples orphan survived: %v", err)
	}
	assertRetirementPairPresent(t, root, "claude", "live")
	evidence, err := ReadCallEvidence(root)
	if err != nil || len(evidence.Samples) != 1 || !evidence.RetainedSince.IsZero() {
		t.Fatalf("orphan-only prune changed evidence=%+v err=%v", evidence, err)
	}
}

func TestCallRetirementRecoversEveryInterruptedDeletion(t *testing.T) {
	tests := []struct {
		name    string
		fail    string
		recover string
	}{
		{"boundary durability unknown", "boundary", "prune"},
		{"journal durability unknown", "journal", "calls"},
		{"samples unlink", "samples", "latest"},
		{"cursor unlink", "cursor", "sessions"},
		{"samples directory sync", "sync-1", "evidence"},
		{"cursor directory sync", "sync-2", "prune"},
		{"journal unlink", "journal-unlink", "calls"},
		{"journal directory sync", "sync-3", "evidence"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			before := retirementTestCutoff()
			old := before.Add(-4 * time.Hour)
			transcript := seedRetirementCall(t, root, "interrupted", old)
			setRetirementPairTimes(t, root, "claude", "interrupted", old, old)
			cursorPath := CursorPath(root, "claude", "interrupted")
			samplesPath := SamplesPath(root, "claude", "interrupted")
			journalPath := callRetirementPath(cursorPath)

			originalWrite := writeCallRetentionText
			originalRemove := removeCallStorePath
			originalSync := syncCallStoreDirectory
			t.Cleanup(func() {
				writeCallRetentionText = originalWrite
				removeCallStorePath = originalRemove
				syncCallStoreDirectory = originalSync
			})
			unlinkCalls := 0
			syncCalls := 0
			writeCallRetentionText = func(path, text, anchor string) (bool, error) {
				durable, err := originalWrite(path, text, anchor)
				if err == nil && durable && (test.fail == "boundary" && path == callRetentionPath(root) || test.fail == "journal" && path == journalPath) {
					return false, nil
				}
				return durable, err
			}
			removeCallStorePath = func(path string) error {
				unlinkCalls++
				if test.fail == "samples" && path == samplesPath || test.fail == "cursor" && path == cursorPath || test.fail == "journal-unlink" && path == journalPath {
					return errors.New("injected unlink failure")
				}
				return originalRemove(path)
			}
			syncCallStoreDirectory = func(path string) error {
				syncCalls++
				if test.fail == fmt.Sprintf("sync-%d", syncCalls) {
					return errors.New("injected sync failure")
				}
				return originalSync(path)
			}

			removed, pruneErr := PruneCallSessions(root, before)
			wantRemoved := 0
			if test.fail == "sync-3" {
				wantRemoved = 1
			}
			if pruneErr == nil || removed != wantRemoved {
				t.Fatalf("interrupted prune removed=%d err=%v", removed, pruneErr)
			}
			if (test.fail == "boundary" || test.fail == "journal") && unlinkCalls != 0 {
				t.Fatalf("durability-unknown publication authorized %d unlinks", unlinkCalls)
			}
			writeCallRetentionText = originalWrite
			removeCallStorePath = originalRemove
			syncCallStoreDirectory = originalSync

			switch test.recover {
			case "prune":
				recovered, err := PruneCallSessions(root, before)
				if err != nil || recovered != 1 {
					t.Fatalf("prune recovery removed=%d err=%v", recovered, err)
				}
			case "calls":
				if _, _, err := Calls(root, "claude", "interrupted", time.Time{}); err != nil {
					t.Fatal(err)
				}
			case "latest":
				reading, err := LatestCall(root, "claude", "interrupted", ReadOptions{Capability: PerCall, Transcript: transcript})
				if err != nil || reading.NewSamples != 1 {
					t.Fatalf("latest recovery=%+v err=%v", reading, err)
				}
			case "sessions":
				if sessions, err := CallSessions(root); err != nil || len(sessions) != 0 {
					t.Fatalf("discovery recovery=%v err=%v", sessions, err)
				}
			case "evidence":
				if evidence, err := ReadCallEvidence(root); err != nil || evidence.RetainedSince.IsZero() {
					t.Fatalf("evidence recovery=%+v err=%v", evidence, err)
				}
			}
			if test.recover == "latest" {
				samples, _, err := Calls(root, "claude", "interrupted", time.Time{})
				if err != nil || len(samples) != 1 || samples[0].InvocationID != "interrupted" {
					t.Fatalf("replacement generation samples=%v err=%v", samples, err)
				}
			} else {
				assertRetirementPairAbsent(t, root, "claude", "interrupted")
			}
			if _, err := os.Lstat(journalPath); !os.IsNotExist(err) {
				t.Fatalf("retirement journal survived recovery: %v", err)
			}
		})
	}
}

func TestCallRetirementDiscoveryFindsJournalOnlyStores(t *testing.T) {
	root := t.TempDir()
	before := retirementTestCutoff()
	old := before.Add(-4 * time.Hour)
	runtimeName, session := "runtime-with-hyphen", "unsafe/session"
	writeDiscoveryCursor(t, root, runtimeName, session)
	appendRetirementRegistration(t, root, CallRegistration{Runtime: runtimeName, Session: session, PID: 10, PIDStartedAt: 20, FirstSeen: old})
	if err := os.Chtimes(CursorPath(root, runtimeName, session), old, old); err != nil {
		t.Fatal(err)
	}

	originalRemove := removeCallStorePath
	journalPath := callRetirementPath(CursorPath(root, runtimeName, session))
	removeCallStorePath = func(path string) error {
		if path == journalPath {
			return errors.New("leave journal for discovery")
		}
		return originalRemove(path)
	}
	if _, err := PruneCallSessions(root, before); err == nil {
		t.Fatal("prune unexpectedly removed its journal")
	}
	removeCallStorePath = originalRemove
	t.Cleanup(func() { removeCallStorePath = originalRemove })
	if _, err := os.Lstat(CursorPath(root, runtimeName, session)); !os.IsNotExist(err) {
		t.Fatalf("cursor survived interrupted retirement: %v", err)
	}
	sessions, err := CallSessions(root)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("journal-only discovery=%v err=%v", sessions, err)
	}
	if _, err := os.Lstat(journalPath); !os.IsNotExist(err) {
		t.Fatalf("journal-only store was not recovered: %v", err)
	}

	t.Run("journal names never replace a lawful cursor", func(t *testing.T) {
		collisionRoot := t.TempDir()
		collisionBefore := retirementTestCutoff()
		collisionOld := collisionBefore.Add(-4 * time.Hour)
		for _, name := range []string{"foo", "foo.json.retiring"} {
			seedRetirementCall(t, collisionRoot, name, collisionOld)
			setRetirementPairTimes(t, collisionRoot, "claude", name, collisionOld, collisionOld)
		}
		ambiguousCursor := CursorPath(collisionRoot, "claude", "foo.json.retiring")
		if !callStorePathHasValidCursor(ambiguousCursor) {
			data, _ := os.ReadFile(ambiguousCursor)
			t.Fatalf("lawful ambiguous cursor was not recognized: %s", data)
		}
		sessions, err := CallSessions(collisionRoot)
		want := []CallSession{{Runtime: "claude", Session: "foo"}, {Runtime: "claude", Session: "foo.json.retiring"}}
		if err != nil || !reflect.DeepEqual(sessions, want) {
			t.Fatalf("collision discovery=%v err=%v", sessions, err)
		}
		if removed, err := PruneCallSessions(collisionRoot, collisionBefore); err == nil || removed != 0 || !strings.Contains(err.Error(), "collides") {
			t.Fatalf("collision prune removed=%d err=%v", removed, err)
		}
		for _, name := range []string{"foo", "foo.json.retiring"} {
			assertRetirementPairPresent(t, collisionRoot, "claude", name)
		}
	})
}

func TestCallRetirementPreservesDamageAndLockInodes(t *testing.T) {
	t.Run("malformed cursor and journal", func(t *testing.T) {
		for _, kind := range []string{"cursor", "journal"} {
			root := t.TempDir()
			before := retirementTestCutoff()
			old := before.Add(-4 * time.Hour)
			seedRetirementCall(t, root, "damaged", old)
			setRetirementPairTimes(t, root, "claude", "damaged", old, old)
			path := CursorPath(root, "claude", "damaged")
			if kind == "journal" {
				path = callRetirementPath(path)
			}
			if err := os.WriteFile(path, []byte("{damaged\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := PruneCallSessions(root, before); err == nil {
				t.Fatalf("%s damage was accepted", kind)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != "{damaged\n" {
				t.Fatalf("%s damage changed: %q err=%v", kind, got, err)
			}
		}
	})

	t.Run("symlinked member", func(t *testing.T) {
		root := t.TempDir()
		before := retirementTestCutoff()
		old := before.Add(-4 * time.Hour)
		seedRetirementCall(t, root, "symlink", old)
		setRetirementPairTimes(t, root, "claude", "symlink", old, old)
		samplesPath := SamplesPath(root, "claude", "symlink")
		target := filepath.Join(t.TempDir(), "outside-evidence")
		if err := os.WriteFile(target, []byte("outside\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(samplesPath); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, samplesPath); err != nil {
			t.Skip(err)
		}
		if _, err := PruneCallSessions(root, before); err == nil {
			t.Fatal("symlinked samples were accepted for retirement")
		}
		if got, err := os.ReadFile(target); err != nil || string(got) != "outside\n" {
			t.Fatalf("symlink target changed: got=%q err=%v", got, err)
		}
		if _, err := os.Lstat(samplesPath); err != nil {
			t.Fatalf("symlinked evidence was removed: %v", err)
		}
	})

	t.Run("symlinked intermediate parent", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(root, "artifacts")); err != nil {
			t.Skip(err)
		}
		if _, err := PruneCallSessions(root, retirementTestCutoff()); err == nil || !strings.Contains(err.Error(), filepath.Join(root, "artifacts")) {
			t.Fatalf("intermediate symlink error=%v", err)
		}
		if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
			t.Fatalf("prune wrote through intermediate symlink: entries=%v err=%v", entries, err)
		}
	})

	t.Run("replacement generation", func(t *testing.T) {
		root := t.TempDir()
		before := retirementTestCutoff()
		old := before.Add(-4 * time.Hour)
		seedRetirementCall(t, root, "replacement", old)
		setRetirementPairTimes(t, root, "claude", "replacement", old, old)
		cursorPath := CursorPath(root, "claude", "replacement")
		originalRemove := removeCallStorePath
		removeCallStorePath = func(path string) error {
			if path == cursorPath {
				return errors.New("stop after samples unlink")
			}
			return originalRemove(path)
		}
		if _, err := PruneCallSessions(root, before); err == nil {
			t.Fatal("retirement was not interrupted")
		}
		removeCallStorePath = originalRemove
		t.Cleanup(func() { removeCallStorePath = originalRemove })
		writeDiscoveryCursor(t, root, "claude", "replacement")
		replacementBytes, err := os.ReadFile(cursorPath)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := Calls(root, "claude", "replacement", time.Time{}); err == nil {
			t.Fatal("replacement cursor satisfied the old retirement journal")
		}
		if got, err := os.ReadFile(cursorPath); err != nil || !reflect.DeepEqual(got, replacementBytes) {
			t.Fatalf("replacement generation changed: err=%v", err)
		}
	})

	t.Run("stable lock inode", func(t *testing.T) {
		root := t.TempDir()
		before := retirementTestCutoff()
		old := before.Add(-4 * time.Hour)
		seedRetirementCall(t, root, "locked", old)
		setRetirementPairTimes(t, root, "claude", "locked", old, old)
		lockPath := CursorPath(root, "claude", "locked") + ".lock"
		lock, err := lockCallFile(lockPath)
		if err != nil {
			t.Fatal(err)
		}
		beforeInfo, err := lock.Stat()
		if err != nil {
			t.Fatal(err)
		}
		attempted := make(chan struct{})
		var once sync.Once
		previousOpen := callFileOpens
		callFileOpens = func(path string) {
			if path == lockPath {
				once.Do(func() { close(attempted) })
			}
		}
		t.Cleanup(func() { callFileOpens = previousOpen })
		done := make(chan error, 1)
		go func() {
			_, err := PruneCallSessions(root, before)
			done <- err
		}()
		<-attempted
		unlockCallFile(lock)
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		afterInfo, err := os.Stat(lockPath)
		if err != nil || !os.SameFile(beforeInfo, afterInfo) {
			t.Fatalf("cursor lock inode changed: err=%v", err)
		}
	})
}

func seedRetirementCall(t *testing.T, root, session string, at time.Time) string {
	t.Helper()
	return seedRetirementCallWithRows(t, root, session, at,
		claudeAssistant(session, 10, 0, 0, false, at.Format(time.RFC3339Nano)))
}

func seedRetirementCallWithRows(t *testing.T, root, session string, at time.Time, rows ...string) string {
	t.Helper()
	transcript := filepath.Join(t.TempDir(), strings.ReplaceAll(session, "/", "-")+".jsonl")
	writeCallRows(t, transcript, rows...)
	if _, err := LatestCall(root, "claude", session, ReadOptions{Capability: PerCall, Transcript: transcript, Now: at}); err != nil {
		t.Fatal(err)
	}
	appendRetirementRegistration(t, root, CallRegistration{
		Runtime: "claude", Session: session, PID: 100, PIDStartedAt: 1000, FirstSeen: at,
	})
	return transcript
}

func setRetirementPairTimes(t *testing.T, root, runtimeName, session string, cursorAt, samplesAt time.Time) {
	t.Helper()
	if err := os.Chtimes(CursorPath(root, runtimeName, session), cursorAt, cursorAt); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(SamplesPath(root, runtimeName, session), samplesAt, samplesAt); err != nil {
		t.Fatal(err)
	}
}

func retirementTestCutoff() time.Time {
	return time.Now().UTC().Add(-callRetentionWindow - 24*time.Hour)
}

func appendRetirementRegistration(t *testing.T, root string, row CallRegistration) {
	t.Helper()
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "context", "sessions.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(append(encoded, '\n'))
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("registration write=%v close=%v", writeErr, closeErr)
	}
}

func assertRetirementPairPresent(t *testing.T, root, runtimeName, session string) {
	t.Helper()
	for _, path := range []string{CursorPath(root, runtimeName, session), SamplesPath(root, runtimeName, session)} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("retained pair member %s: %v", path, err)
		}
	}
}

func assertRetirementPairAbsent(t *testing.T, root, runtimeName, session string) {
	t.Helper()
	for _, path := range []string{CursorPath(root, runtimeName, session), SamplesPath(root, runtimeName, session)} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("retired pair member %s survived: %v", path, err)
		}
	}
}
