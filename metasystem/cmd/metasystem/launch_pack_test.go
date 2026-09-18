package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestPackCheckVerbPrintsOneLine(t *testing.T) {
	templates := filepath.Join(t.TempDir(), "templates")
	if err := os.MkdirAll(templates, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templates, "design-brief.md"), []byte("Design <N>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(templates, "review-brief.md"), []byte("Review <N>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &launch.Manager{Store: launch.Store{Root: t.TempDir()}}
	field := reflect.ValueOf(manager).Elem().FieldByName("TemplateDirectory")
	if field.IsValid() {
		field.SetString(templates)
	}
	old := launchManager
	launchManager = func() *launch.Manager { return manager }
	t.Cleanup(func() { launchManager = old })

	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "source.txt"), []byte("one\ntwo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	filled := filepath.Join(t.TempDir(), "filled.md")
	if err := os.WriteFile(filled, []byte("Design ready\n`source.txt:1-2`\n\n   ```text\n   one\n   two\n   ```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	packCheck := routedLaunchPackCheck(t)
	code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
		return packCheck([]string{"--kind", "design", "--brief", filled, "--dir", directory})
	})
	if code != 0 || stdout != "pack=ok kind=design ranges=1\n" || stderr != "" {
		t.Fatalf("ok code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	unfilled := filepath.Join(t.TempDir(), "unfilled.md")
	if err := os.WriteFile(unfilled, []byte("Review <N>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = captureCommandOutput(t, true, true, func() int {
		return packCheck([]string{"--kind", "read", "--brief", unfilled, "--dir", directory})
	})
	if code != 1 || !strings.HasPrefix(stdout, "LAUNCH_BRIEF_PACK_UNFILLED kind=read\n") || !strings.Contains(stdout, "placeholder=<N> line=1\n") || stderr != "" {
		t.Fatalf("refused code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	refusals, err := manager.Store.Refusals()
	if err != nil || len(refusals) != 0 {
		t.Fatalf("pack-check wrote refusals=%+v err=%v", refusals, err)
	}
}

func routedLaunchPackCheck(t *testing.T) func([]string) int {
	t.Helper()
	for _, family := range families() {
		if family.name != "launch" {
			continue
		}
		for _, verb := range family.verbs {
			if verb.name == "pack-check" {
				return verb.run
			}
		}
	}
	t.Fatal("launch pack-check verb is not registered")
	return nil
}
