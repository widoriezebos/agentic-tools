package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adopt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// The public action reaches adoption's configuration tailoring before goal
// genesis. Genesis stops the fixture before registration and hook installation.
func TestSystemAdoptDropsTemplateProofDeclarationsBeforeGenesis(t *testing.T) {
	t.Parallel()
	source, owners := newHomesSettingsInstallation(t)
	target := filepath.Join(t.TempDir(), "app")
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for path, body := range map[string]string{
		"metasystem.conf": "# template mode\nmetasystem.template=true\n# full proof\nproof.full=template-full\nproof.cheap=template-cheap\nproof.other=template-only\nmetasystem.version=1\n",
		"testing.json":    "{}\n", "cmd/metasystem/main.go": "package main\n", "docs/project-rules.md": "# Project rules\n",
	} {
		if err := writer.WriteHeader(&tar.Header{Name: path, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("a", 40)
	stopped := errors.New("fixture stops after checking the copied configuration")
	checked := false
	owners.resolver = stateroot.NewResolver(fakeTop(source), noExecutable)
	owners.adopt = func(options adopt.Options) (adopt.Result, error) {
		options.Deps = adopt.Deps{
			LookPath:    func(string) (string, error) { return "/fixture/tool", nil },
			EngineStamp: sha,
			Build: func(string) error {
				if err := os.MkdirAll(filepath.Join(source, "bin"), 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(source, "bin", "metasystem"), []byte("fixture engine"), 0o755)
			},
			Git: func(_ string, _ bool, args ...string) ([]byte, error) {
				switch strings.Join(args, " ") {
				case "rev-parse --is-inside-work-tree":
					return []byte("true\n"), nil
				case "status --porcelain -- .", "rev-parse --show-prefix":
					return nil, nil
				case "rev-parse HEAD":
					return []byte(sha), nil
				case "archive HEAD":
					return archive.Bytes(), nil
				case "config --get metasystem.goal.machine":
					return []byte("fixture"), nil
				case "rev-parse --path-format=absolute --git-path hooks", "rev-parse --git-dir":
					return nil, errors.New("no hooks or Git setup in fixture")
				}
				t.Errorf("unexpected stub Git: %v", args)
				return nil, errors.New("unexpected Git")
			},
			Genesis: func(root string) error {
				content, err := os.ReadFile(filepath.Join(root, "metasystem.conf"))
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(string(content), "proof.") || strings.Contains(string(content), "metasystem.template=") || !strings.Contains(string(content), "metasystem.version=1") {
					t.Fatalf("template declarations survived adoption or project setting was lost: %s", content)
				}
				checked = true
				return stopped
			},
		}
		return adopt.Adopt(options)
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(mustIntentCommand(t, "system adopt"), []string{target, "--repo", source, "--runtimes", "none"}, &stdout, &stderr, source, owners)
	if !checked || code != 1 || !strings.Contains(stdout.String()+stderr.String(), stopped.Error()) {
		t.Fatalf("adopt did not reach the copied configuration: %d %s%s checked=%v", code, &stdout, &stderr, checked)
	}
}
