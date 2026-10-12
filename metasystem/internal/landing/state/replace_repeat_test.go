package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReplaceEmptyDiffPreservesBytesAndModificationTimes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Date(2026, 10, 12, 20, 0, 0, 0, time.UTC)
	initial := Record{Lane: Lane{Fact: Fact{Word: "running"}}}
	if _, err := replaceFixture(dir, now, initial, "fixture"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"state.json", "events.jsonl", "state.lock"} {
		path := filepath.Join(dir, name)
		if err := os.Chtimes(path, now, now); err != nil {
			t.Fatal(err)
		}
	}
	before := make(map[string]string)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		before[entry.Name()] = string(data)
	}
	changed, err := replaceFixture(dir, now.Add(time.Minute), initial, "keeper")
	if err != nil || len(changed) != 0 {
		t.Fatalf("empty diff: changed=%v error=%v", changed, err)
	}
	entries, err = os.ReadDir(dir)
	if err != nil || len(entries) != len(before) {
		t.Fatalf("empty diff changed file membership: %v %v", entries, err)
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil || string(data) != before[entry.Name()] {
			t.Fatalf("empty diff changed %s: %s %v", entry.Name(), data, err)
		}
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().Equal(now) {
			t.Fatalf("empty diff touched %s: modification time=%s", entry.Name(), info.ModTime())
		}
	}
}
