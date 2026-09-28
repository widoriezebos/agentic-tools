package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// u6bPortTreeSnapshot is every file under root with its bytes.
func u6bPortTreeSnapshot(t *testing.T, root string) string {
	t.Helper()
	var lines []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lines = append(lines, path+"\x00"+string(content))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(lines)
	return strings.Join(lines, "\x01")
}

// Port of dispatch-fixtures.sh seat-refused (lines 1153-1213): a delegate
// that reaches `system stop` for its own checkout (installation at the
// checkout top, so the retry names no --installation) is refused with the
// exact two-line terminal remedy and exit 1; `system status` from the same
// seat prints the exact empty inventory; neither changes any file of the
// checkout. The DELEGATE verdict itself is the lease classifier's
// (internal/lease/classify_test.go:TestClassifyDelegateThroughSignedAncestor).
func TestU6bPortDelegateStopIsRefusedAndStatusIsReadOnly(t *testing.T) {
	repo, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"bin/metasystem":                          "fixture engine\n",
		"metasystem.conf":                         "metasystem.runtimes=fake\n",
		"scripts/agents/adapters/fake.sh":         "#!/bin/sh\nprintf 'match fake-runtime-never-present\\n'\n",
		"artifacts/agents/jobs/earlier.json":      `{"jobId":"earlier","status":"completed","chainClosed":true}` + "\n",
		"artifacts/agents/supervision/state.json": `{"generation":1}` + "\n",
		"artifacts/agents/steward/identity.json":  `{"enrollment":"fixture"}` + "\n",
	} {
		full := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := testexec.WriteFile(full, []byte(content), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	processes := filepath.Join(t.TempDir(), "processes.json")
	if err := os.WriteFile(processes, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_CENSUS_PROCESS_FILE", processes)
	before := u6bPortTreeSnapshot(t, repo)
	repositoryTop := declaredRepositoryTop(t, repo, map[string]int{repo: 3})
	classify := func(string, string, int64) (lease.Classification, error) {
		return lease.Classification{Class: lease.ClassDelegate}, nil
	}

	stdout, stderr, code := captureRelay(t, func() int {
		return runProcessStopWith([]string{"--repo", repo}, repositoryTop, classify)
	})
	want := "metasystem system stop: system stop is a human act at a terminal; this caller is DELEGATE.\n" +
		"at an agent-free terminal, run: metasystem system stop --repo " + repo + "\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Fatalf("delegate stop = code %d stdout %q stderr %q, want exit 1 and %q", code, stdout, stderr, want)
	}

	stdout, stderr, code = captureRelay(t, func() int {
		return runProcessStatusWith([]string{"--repo", repo}, repositoryTop)
	})
	if code != 0 || stderr != "" || stdout != "checkout "+repo+"\nnothing is running\n" {
		t.Fatalf("delegate status = code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if after := u6bPortTreeSnapshot(t, repo); after != before {
		t.Fatalf("the refused stop or the read-only status changed the checkout:\nbefore %q\nafter  %q",
			bytes.ReplaceAll([]byte(before), []byte{1}, []byte("\n")), bytes.ReplaceAll([]byte(after), []byte{1}, []byte("\n")))
	}
}
