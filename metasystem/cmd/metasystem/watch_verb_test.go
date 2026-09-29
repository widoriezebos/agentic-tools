package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	metarun "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestWaitCompatibilityMappings(t *testing.T) {
	original := compatibilityWaitCommand
	defer func() { compatibilityWaitCommand = original }()

	cases := []struct {
		code    int
		outcome string
	}{
		{0, "green"}, {1, "red"}, {2, "ended-unknown"}, {3, "launch-failed"}, {4, "target-replaced"},
		{64, "registration-refused"}, {65, "storage-failure"}, {66, "uncertain-identity"},
		{67, "invalid-arguments"}, {124, "wait-deadline"}, {130, "interrupted"},
	}
	for _, test := range cases {
		compatibilityWaitCommand = func(_ []string, _ func(context.Context) error, _ int64, printResult func(metarun.WaitResult, bool), _, _ io.Writer) int {
			printResult(metarun.WaitResult{ExitCode: test.code, SourceOutcome: test.outcome, TargetIncarnation: metarun.WaiterTarget{Generation: 1}}, false)
			return test.code
		}
		if output, code := captureStdout(t, func(stdout, stderr io.Writer) int {
			return runRunWatch([]string{"--root", t.TempDir(), "--id", "compat", "--caller-pid", "1"}, stdout, stderr)
		}); code != test.code {
			t.Fatalf("run watch wait exit %d mapped to %d (output %q)", test.code, code, output)
		}
		if code := runJobWatchVerb([]string{"--root", t.TempDir(), "--job", "compat", "--caller-pid", "1"}, t.Output(), t.Output()); code != test.code {
			t.Fatalf("job watch wait exit %d mapped to %d", test.code, code)
		}
	}
	// The delegate --wait mapping of these exits (0/1/2/3/4 -> 0/3/4/8/5)
	// is the delegation lifecycle's: internal/delegation
	// TestWaitReapsOnlyThroughTheLeaseHeldEntry.
}

func watchWriteJSON(t *testing.T, root, relative string, value any) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func watchTreeHash(t *testing.T, root string) string {
	t.Helper()
	paths := []string{}
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(hash, "%s\x00%s\x00", filepath.ToSlash(relative), info.Mode())
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			hash.Write(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			hash.Write([]byte(target))
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}
