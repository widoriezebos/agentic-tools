package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathclass"
)

func TestPathStateRootVerbPrintsTheValidatedInstallation(t *testing.T) {
	outer := t.TempDir()
	installation := filepath.Join(outer, "metasystem")
	if err := os.MkdirAll(filepath.Join(installation, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(outer, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outer, "development", "metasystem-design.md"), []byte("design\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := captureStdout(t, func() int { return runPathStateRoot([]string{installation}) })
	want, err := filepath.EvalSymlinks(installation)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || out != want+"\n" {
		t.Fatalf("path state-root returned code=%d stdout=%q; want code=0 stdout=%q", code, out, want+"\n")
	}
}

func TestPathStateRootVerbRefusesUsageAndInvalidCandidates(t *testing.T) {
	stderr, code := captureStderr(t, func() int { return runPathStateRoot(nil) })
	if code != 2 || !strings.Contains(stderr, "usage: metasystem path state-root") {
		t.Fatalf("missing installation: code=%d stderr=%q", code, stderr)
	}
	stderr, code = captureStderr(t, func() int { return runPathStateRoot([]string{"one", "two"}) })
	if code != 2 || !strings.Contains(stderr, "usage: metasystem path state-root") {
		t.Fatalf("extra installation: code=%d stderr=%q", code, stderr)
	}
	stderr, code = captureStderr(t, func() int { return runPathStateRoot([]string{t.TempDir()}) })
	if code != 1 || !strings.Contains(stderr, "not a metasystem installation") {
		t.Fatalf("invalid installation: code=%d stderr=%q", code, stderr)
	}
}

func TestPathClassVerbOneWordAndRefusalText(t *testing.T) {
	original := resolvePathClass
	t.Cleanup(func() { resolvePathClass = original })

	for _, class := range []pathclass.Class{pathclass.Behavior, pathclass.Record, pathclass.Ledger, pathclass.Runtime} {
		t.Run(string(class), func(t *testing.T) {
			resolvePathClass = func(string) (pathclass.Resolution, error) {
				return pathclass.Resolution{Class: class, Namespace: pathclass.Install, Key: "fixture", Row: "install:fixture", Mode: pathclass.Template}, nil
			}
			out, code := captureStdout(t, func() int { return runPathClass([]string{"fixture"}) })
			if code != 0 || out != string(class)+"\n" {
				t.Fatalf("path class returned code=%d stdout=%q; want code=0 stdout=%q", code, out, class+"\n")
			}
		})
	}

	resolvePathClass = func(string) (pathclass.Resolution, error) {
		return pathclass.Resolution{Class: pathclass.Unclassified, Namespace: pathclass.Install, Key: "product.txt", Mode: pathclass.Template}, nil
	}
	var stdout string
	stderr, code := captureStderr(t, func() int {
		var innerCode int
		stdout, innerCode = captureStdout(t, func() int { return runPathClass([]string{"product.txt"}) })
		return innerCode
	})
	wantRefusal := "path product.txt has no class in scripts/agents/path-classes.txt; no classified ancestor; add a row for product.txt or its directory to scripts/agents/path-classes.txt\n"
	if code != 1 || stdout != "unclassified\n" || stderr != wantRefusal {
		t.Fatalf("unclassified answer: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestPathClassVerbAnswersOutsideAndExplainsMatch(t *testing.T) {
	original := resolvePathClass
	t.Cleanup(func() { resolvePathClass = original })

	resolvePathClass = func(string) (pathclass.Resolution, error) {
		return pathclass.Resolution{Class: pathclass.Outside, Mode: pathclass.Adopted}, nil
	}
	out, code := captureStdout(t, func() int { return runPathClass([]string{"../outside"}) })
	if code != 1 || out != "outside\n" {
		t.Fatalf("outside answer: code=%d stdout=%q", code, out)
	}

	resolvePathClass = func(string) (pathclass.Resolution, error) {
		return pathclass.Resolution{
			Class: pathclass.Behavior, Namespace: pathclass.Install, Key: "docs/guide.md",
			Row: "install:docs/", Mode: pathclass.Template,
		}, nil
	}
	out, code = captureStdout(t, func() int { return runPathClass([]string{"--explain", "metasystem/docs/guide.md"}) })
	if code != 0 || out != "behavior row=install:docs/ key=install:docs/guide.md mode=template\n" {
		t.Fatalf("explained answer: code=%d stdout=%q", code, out)
	}
}

func TestPathClassVerbUsageAndResolverFailure(t *testing.T) {
	original := resolvePathClass
	t.Cleanup(func() { resolvePathClass = original })

	stderr, code := captureStderr(t, func() int { return runPathClass(nil) })
	if code != 2 || !strings.Contains(stderr, "usage: metasystem path class") {
		t.Fatalf("missing path: code=%d stderr=%q", code, stderr)
	}
	resolvePathClass = func(string) (pathclass.Resolution, error) {
		return pathclass.Resolution{}, errors.New("manifest unavailable")
	}
	stderr, code = captureStderr(t, func() int { return runPathClass([]string{"x"}) })
	if code != 1 || stderr != "manifest unavailable\n" {
		t.Fatalf("resolver failure: code=%d stderr=%q", code, stderr)
	}
}
