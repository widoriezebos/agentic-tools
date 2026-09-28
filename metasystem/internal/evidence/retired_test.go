package evidence

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestWriteRetiredIsIdempotent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "RETIRED.json")
	first := time.Date(2026, 9, 28, 20, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	read := func() []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	changed, err := WriteRetired(root, []string{"/c/b", "/c/a", "/c/b"}, first)
	if err != nil || !changed {
		t.Fatalf("first write: %v, %v", changed, err)
	}
	var pointer map[string]any
	if err := json.Unmarshal(read(), &pointer); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"schemaVersion": float64(1), "retiredAt": "2026-09-28T18:00:00Z",
		"checkouts": []any{"/c/a", "/c/b"}, "successor": "per-checkout default", "rule": "evidence-root-default",
	}
	if !reflect.DeepEqual(pointer, want) {
		t.Fatalf("pointer = %v\nwant %v", pointer, want)
	}
	if !bytes.HasPrefix(read(), []byte(`{"schemaVersion":1,"retiredAt":"2026-09-28T18:00:00Z","checkouts":["/c/a","/c/b"],"successor":"per-checkout default","rule":"evidence-root-default"}`)) {
		t.Fatalf("pointer bytes = %s", read())
	}

	before := read()
	changed, err = WriteRetired(root, []string{"/c/a", "/c/b"}, first.Add(time.Hour))
	if err != nil || changed || !bytes.Equal(before, read()) {
		t.Fatalf("a rerun changed the pointer: %v, %v\n%s", changed, err, read())
	}

	changed, err = WriteRetired(root, []string{"/c/c"}, first.Add(2*time.Hour))
	if err != nil || !changed {
		t.Fatalf("a new checkout: %v, %v", changed, err)
	}
	if err := json.Unmarshal(read(), &pointer); err != nil {
		t.Fatal(err)
	}
	if pointer["retiredAt"] != "2026-09-28T18:00:00Z" || !reflect.DeepEqual(pointer["checkouts"], []any{"/c/a", "/c/b", "/c/c"}) {
		t.Fatalf("after adding: %v", pointer)
	}

	before = read()
	changed, err = WriteRetired(root, []string{"/c/b"}, first.Add(3*time.Hour))
	if err != nil || changed || !bytes.Equal(before, read()) {
		t.Fatalf("a subset removed or rewrote: %v, %v\n%s", changed, err, read())
	}
}

func TestWriteRetiredRefusesWhatIsNotAPointer(t *testing.T) {
	t.Parallel()
	if _, err := WriteRetired("relative", []string{"/c/a"}, time.Now()); err == nil {
		t.Fatal("a relative root was accepted")
	}
	root := t.TempDir()
	if _, err := WriteRetired(root, []string{"relative"}, time.Now()); err == nil {
		t.Fatal("a relative checkout was accepted")
	}
	if _, err := WriteRetired(root, nil, time.Now()); err == nil {
		t.Fatal("no checkout was accepted")
	}
	if err := os.WriteFile(filepath.Join(root, "RETIRED.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteRetired(root, []string{"/c/a"}, time.Now()); err == nil {
		t.Fatal("an unreadable pointer was overwritten")
	}
	missing := filepath.Join(t.TempDir(), "gone")
	if _, err := WriteRetired(missing, []string{"/c/a"}, time.Now()); err == nil {
		t.Fatal("a pointer was written into a root that does not exist")
	}
}
