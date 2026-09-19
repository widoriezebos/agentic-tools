package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCallRegistrationsReadsStrictSnapshot(t *testing.T) {
	t.Run("absolute state root is required", func(t *testing.T) {
		if _, _, err := CallRegistrations("relative"); err == nil || !strings.Contains(err.Error(), "state root must be absolute") {
			t.Fatalf("relative root error = %v", err)
		}
	})

	t.Run("missing and empty registry are distinguished", func(t *testing.T) {
		stateRoot := t.TempDir()
		rows, present, err := CallRegistrations(stateRoot)
		if err != nil || present || len(rows) != 0 {
			t.Fatalf("missing registry = rows %#v, present %t, err %v", rows, present, err)
		}
		writeRegistrationSnapshot(t, stateRoot, "")
		rows, present, err = CallRegistrations(stateRoot)
		if err != nil || !present || len(rows) != 0 {
			t.Fatalf("empty registry = rows %#v, present %t, err %v", rows, present, err)
		}
	})

	t.Run("complete rows preserve registry order and fields", func(t *testing.T) {
		stateRoot := t.TempDir()
		want := []CallRegistration{
			{Runtime: "claude", Session: "first", PID: 101, PIDStartedAt: 1001, FirstSeen: time.Date(2026, 9, 13, 1, 2, 3, 4, time.UTC)},
			{Runtime: "codex", Session: "second/session", PID: 102, PIDStartedAt: 1002, FirstSeen: time.Date(2026, 9, 14, 5, 6, 7, 8, time.UTC)},
		}
		var content strings.Builder
		for _, row := range want {
			encoded, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			content.Write(encoded)
			content.WriteByte('\n')
		}
		writeRegistrationSnapshot(t, stateRoot, content.String())
		got, present, err := CallRegistrations(stateRoot)
		if err != nil || !present || !reflect.DeepEqual(got, want) {
			t.Fatalf("complete registry = rows %#v, present %t, err %v; want %#v", got, present, err, want)
		}
	})

	t.Run("incomplete and malformed rows are refused", func(t *testing.T) {
		valid, err := json.Marshal(CallRegistration{
			Runtime: "claude", Session: "valid", PID: 103, PIDStartedAt: 1003,
			FirstSeen: time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct {
			name, content, want string
		}{
			{"incomplete tail", string(valid), "incomplete row"},
			{"malformed json", "{broken\n", "malformed"},
			{"multiple json values", string(valid) + " {}\n", "malformed"},
			{"incomplete fields", "{\"runtime\":\"claude\"}\n", "incomplete"},
		} {
			t.Run(test.name, func(t *testing.T) {
				stateRoot := t.TempDir()
				path := writeRegistrationSnapshot(t, stateRoot, test.content)
				rows, present, err := CallRegistrations(stateRoot)
				if err == nil || !present || len(rows) != 0 || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("strict read = rows %#v, present %t, err %v", rows, present, err)
				}
			})
		}
	})

	t.Run("nonregular registry is refused", func(t *testing.T) {
		stateRoot := t.TempDir()
		path := filepath.Join(stateRoot, "artifacts", "agents", "context", "sessions.jsonl")
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		rows, present, err := CallRegistrations(stateRoot)
		if err == nil || !present || len(rows) != 0 || !strings.Contains(err.Error(), "not a regular file") || !strings.Contains(err.Error(), path) {
			t.Fatalf("nonregular registry = rows %#v, present %t, err %v", rows, present, err)
		}
	})
}

func TestCallSessionsDiscoversPairsWithoutReadingSamples(t *testing.T) {
	t.Run("safe hashed and hyphenated identities", func(t *testing.T) {
		stateRoot := t.TempDir()
		for _, session := range []string{"high.tmp", "safe-session", "unsafe/session"} {
			transcript := filepath.Join(t.TempDir(), "transcript.jsonl")
			writeCallRows(t, transcript, claudeAssistant(session, 10, 0, 0, false, "2026-09-13T10:00:00Z"))
			if _, err := LatestCall(stateRoot, "claude", session, ReadOptions{Capability: PerCall, Transcript: transcript}); err != nil {
				t.Fatal(err)
			}
		}
		writeDiscoveryCursor(t, stateRoot, "runtime-with-hyphen", "session-three")
		emptyOrphan := filepath.Join(stateRoot, "artifacts", "agents", "context", "samples", "orphan-empty.jsonl")
		if err := os.MkdirAll(filepath.Dir(emptyOrphan), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(emptyOrphan, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, ignored := range []string{
			filepath.Join(filepath.Dir(emptyOrphan), "ignored.jsonl.lock"),
			filepath.Join(filepath.Dir(emptyOrphan), "ignored.jsonl.123.tmp"),
			filepath.Join(filepath.Dir(CursorPath(stateRoot, "claude", "safe-session")), "ignored.json.123.tmp"),
		} {
			if err := os.WriteFile(ignored, []byte("ignored"), 0o644); err != nil {
				t.Fatal(err)
			}
		}

		var opened []string
		previous := callFileOpens
		callFileOpens = func(path string) { opened = append(opened, path) }
		t.Cleanup(func() { callFileOpens = previous })
		sessions, err := CallSessions(stateRoot)
		if err != nil {
			t.Fatal(err)
		}
		want := []CallSession{
			{Runtime: "claude", Session: "high.tmp"},
			{Runtime: "claude", Session: "safe-session"},
			{Runtime: "claude", Session: "unsafe/session"},
			{Runtime: "runtime-with-hyphen", Session: "session-three"},
		}
		if !reflect.DeepEqual(sessions, want) {
			t.Fatalf("sessions = %#v, want %#v", sessions, want)
		}
		for _, path := range opened {
			if strings.Contains(path, string(filepath.Separator)+"samples"+string(filepath.Separator)) {
				t.Fatalf("discovery opened a sample body: %s", path)
			}
		}
	})

	t.Run("malformed cursor", func(t *testing.T) {
		stateRoot := t.TempDir()
		path := CursorPath(stateRoot, "claude", "broken")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{broken\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := CallSessions(stateRoot); err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "malformed") {
			t.Fatalf("malformed cursor error = %v", err)
		}
	})

	t.Run("nonempty orphan", func(t *testing.T) {
		stateRoot := t.TempDir()
		samplesPath := SamplesPath(stateRoot, "claude", "orphan")
		if err := os.MkdirAll(filepath.Dir(samplesPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(samplesPath, []byte("evidence\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := CallSessions(stateRoot)
		if err == nil || !strings.Contains(err.Error(), samplesPath) || !strings.Contains(err.Error(), CursorPath(stateRoot, "claude", "orphan")) {
			t.Fatalf("orphan error = %v", err)
		}
	})

	t.Run("nonregular and unreadable members", func(t *testing.T) {
		for _, kind := range []string{"symlink", "unreadable"} {
			t.Run(kind, func(t *testing.T) {
				stateRoot := t.TempDir()
				writeDiscoveryCursor(t, stateRoot, "claude", kind)
				samplesPath := SamplesPath(stateRoot, "claude", kind)
				if err := os.MkdirAll(filepath.Dir(samplesPath), 0o755); err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "symlink":
					target := filepath.Join(t.TempDir(), "target")
					if err := os.WriteFile(target, nil, 0o644); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, samplesPath); err != nil {
						t.Skip(err)
					}
				case "unreadable":
					if err := os.WriteFile(samplesPath, nil, 0o000); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := CallSessions(stateRoot); err == nil || !strings.Contains(err.Error(), samplesPath) {
					t.Fatalf("%s member error = %v", kind, err)
				}
			})
		}
	})

	t.Run("concurrent first publication", func(t *testing.T) {
		stateRoot := t.TempDir()
		runtimeName, session := "claude", "concurrent"
		cursorPath := CursorPath(stateRoot, runtimeName, session)
		samplesPath := SamplesPath(stateRoot, runtimeName, session)
		if err := os.MkdirAll(filepath.Dir(samplesPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(samplesPath, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		lock, err := lockCallFile(cursorPath + ".lock")
		if err != nil {
			t.Fatal(err)
		}
		attempted := make(chan struct{}, 1)
		previous := callFileOpens
		callFileOpens = func(path string) {
			if path == cursorPath+".lock" {
				attempted <- struct{}{}
			}
		}
		t.Cleanup(func() { callFileOpens = previous })
		type result struct {
			sessions []CallSession
			err      error
		}
		finished := make(chan result, 1)
		go func() {
			sessions, err := CallSessions(stateRoot)
			finished <- result{sessions: sessions, err: err}
		}()
		<-attempted
		writeDiscoveryCursor(t, stateRoot, runtimeName, session)
		unlockCallFile(lock)
		got := <-finished
		if got.err != nil || !reflect.DeepEqual(got.sessions, []CallSession{{Runtime: runtimeName, Session: session}}) {
			t.Fatalf("concurrent discovery = %#v, %v", got.sessions, got.err)
		}
	})
}

func writeDiscoveryCursor(t *testing.T, stateRoot, runtimeName, session string) {
	t.Helper()
	stream := filepath.Join(t.TempDir(), "stream.jsonl")
	if err := os.WriteFile(stream, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(stream)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("stream has no syscall identity")
	}
	cursor := freshCallCursor(runtimeName, session, stream)
	cursor.Dev = uint64(stat.Dev)
	cursor.Inode = uint64(stat.Ino)
	cursor.Size = info.Size()
	cursor.Offset = info.Size()
	cursor.LastReadAt = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	if err := writeCallCursor(CursorPath(stateRoot, runtimeName, session), cursor); err != nil {
		t.Fatal(err)
	}
}

func writeRegistrationSnapshot(t *testing.T, stateRoot, content string) string {
	t.Helper()
	path := filepath.Join(stateRoot, "artifacts", "agents", "context", "sessions.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
