package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPushesPreservesDamageCount(t *testing.T) {
	t.Parallel()
	install := t.TempDir()
	if err := os.MkdirAll(Dir(install), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pushesPath(install), []byte("{\"commit\":\"a\",\"old\":\"base\"}\n{\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pushes, damaged, err := Pushes(install)
	if err != nil || damaged != 1 || len(pushes) != 1 || pushes[0].Commit != "a" || pushes[0].Old != "base" {
		t.Fatalf("pushes = %+v, damaged %d, error %v", pushes, damaged, err)
	}
}

func TestResultsReadLongLinesAndPreserveDamage(t *testing.T) {
	t.Parallel()
	for _, ending := range []string{"", "\n", "\r\n"} {
		t.Run(fmt.Sprintf("ending-%q", ending), func(t *testing.T) {
			t.Parallel()
			install := t.TempDir()
			if err := os.MkdirAll(Dir(install), 0700); err != nil {
				t.Fatal(err)
			}
			long := Result{Attempt: "long", Reason: strings.Repeat("x", 6*1024*1024)}
			encoded, err := json.Marshal(long)
			if err != nil {
				t.Fatal(err)
			}
			data := append([]byte("\n \t\r\n{damaged\n"), encoded...)
			data = append(data, []byte("\r\n[wrong type]\n{\"attempt\":\"normal\"}"+ending)...)
			if err := os.WriteFile(resultsPath(install), data, 0600); err != nil {
				t.Fatal(err)
			}
			results, err := Results(install)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != 2 || results[0].Reason != long.Reason || results[1].Attempt != "normal" {
				t.Fatalf("read %d results; want the complete long result and normal result", len(results))
			}
			_, ordinals, damaged, err := countedLineOrdinals[Result](resultsPath(install))
			if err != nil || damaged != 2 || !slices.Equal(ordinals, []int{4, 6}) {
				t.Fatalf("ordinals %v, damaged %d, error %v", ordinals, damaged, err)
			}
		})
	}
}

func TestCountedLinesPreserveEmptyDamagedAndReadErrors(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, data string
		damaged    int
	}{
		{"empty", "", 0}, {"blank", "\n \t\r\n", 0},
		{"torn final", "{", 1}, {"two damaged", "{\n[1]", 2},
		{"long damaged", strings.Repeat("x", 6*1024*1024) + "\n{", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "records.jsonl")
			if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			rows, damaged, err := countedLines[Result](path)
			if len(rows) != 0 || damaged != tc.damaged || err != nil {
				t.Fatalf("rows %d, damaged %d, error %v", len(rows), damaged, err)
			}
		})
	}
	dir := t.TempDir()
	rows, damaged, err := countedLines[Result](filepath.Join(dir, "missing"))
	if rows != nil || damaged != 0 || err != nil {
		t.Fatalf("missing file: %v %d %v", rows, damaged, err)
	}
	rows, damaged, err = countedLines[Result](dir)
	if rows != nil || damaged != 0 || err == nil {
		t.Fatalf("directory: %v %d %v", rows, damaged, err)
	}
}
