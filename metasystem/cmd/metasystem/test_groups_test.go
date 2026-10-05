package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/pathpattern"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestTestGroupsPrintsLandingLines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range contract.Groups {
		group := &contract.Groups[i]
		if group.ID != "verb-ratchet" && group.ID != "agent-protocol-standard" {
			continue
		}
		status := "0"
		diagnostic := "green"
		if group.ID == "agent-protocol-standard" {
			status = "1"
			diagnostic = "red"
		}
		*group = testpolicy.Group{ID: group.ID, Kind: "build", Adapter: "command", CWD: ".", Phase: "acceptance", EnvironmentMode: "inherit",
			Inputs: []string{"go.mod"}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"/bin/sh", "-c", "printf '" + diagnostic + " stdout\\n'; printf '" + diagnostic + " stderr\\n' >&2; exit " + status}, Format: "exit-status"}
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"metasystem.conf": []byte("testing.contract=testing.json\n"), "testing.json": data} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tools := t.TempDir()
	for _, name := range []string{"go", "uname"} {
		executable, err := exec.LookPath(name)
		if err != nil || os.Symlink(executable, filepath.Join(tools, name)) != nil {
			t.Fatalf("prepare %s: %v", name, err)
		}
	}
	environment := proofrun.TestingEnvironment(os.Environ(), map[string]string{"PATH": tools, "GOTOOLCHAIN": "local"})
	var stdout, stderr, combined bytes.Buffer
	run := func(args ...string) int {
		stdout.Reset()
		stderr.Reset()
		combined.Reset()
		return runTestGroupsWithEnvironment(append(args, "--root", root), environment, io.MultiWriter(&stdout, &combined), io.MultiWriter(&stderr, &combined))
	}
	if code := run("verb-ratchet", "agent-protocol-standard", "--environment"); code != 1 || stderr.String() != "red stdout\nred stderr\n" {
		t.Fatalf("exit %d, stderr %s, stdout %s", code, &stderr, &stdout)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 4 || !strings.HasPrefix(lines[0], "landing environment go version ") ||
		!regexp.MustCompile(`^landing group verb-ratchet green [0-9]+$`).MatchString(lines[1]) ||
		!regexp.MustCompile(`^landing group agent-protocol-standard red [0-9]+$`).MatchString(lines[2]) ||
		lines[3] != "landing group agent-protocol-standard reason exit status 1" {
		t.Fatalf("landing lines: %q", lines)
	}
	if !strings.HasSuffix(combined.String(), lines[3]+"\n"+stderr.String()) {
		t.Fatalf("diagnostics must follow the group's reason lines: %q", combined.String())
	}
	description := lines[0]
	for _, name := range []string{"GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS", "GOEXPERIMENT", "GOTOOLCHAIN"} {
		if !strings.Contains(description, name+"=") {
			t.Errorf("environment omitted %s: %s", name, description)
		}
	}
	if code := run("--environment"); code != 0 || stdout.String() != description+"\n" || stderr.Len() != 0 {
		t.Fatalf("environment-only exit %d, out %s, err %s", code, &stdout, &stderr)
	}
	if code := run("verb-ratchet"); code != 0 || stderr.Len() != 0 {
		t.Fatalf("green group exit %d: %s", code, &stderr)
	}
	for _, args := range [][]string{nil, {"unknown"}, {"go-affected"}, {"verb-ratchet", "--wrong"}} {
		if code := run(args...); code == 0 || stderr.Len() == 0 || strings.Contains(stdout.String(), "landing group ") {
			t.Fatalf("refusal %v: exit %d, out %s, err %s", args, code, &stdout, &stderr)
		}
	}
}

func TestPlansAndLedgerMovesSelectAtMostFourGroups(t *testing.T) {
	t.Parallel()
	contract, err := testpolicy.Load(filepath.Join("..", "..", "testing.json"))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"metasystem/plans/designs/x.md", "metasystem/plans/goals/x.md", "metasystem/plans/goals.md",
		"metasystem/plans/goals-accepted.json", "metasystem/plans/x-brief.md", "metasystem/records/goals/x.md",
		"metasystem/records/counselor/x.json", "metasystem/memory/receipts.log", "metasystem/records/narrator-digest.log",
	}
	selections := [][]string{paths}
	for _, path := range paths {
		selections = append(selections, []string{path})
	}
	for _, paths := range selections {
		affected, err := testpolicy.Affected(contract, paths)
		if err != nil {
			t.Fatal(err)
		}
		if len(affected.Uncovered) != 0 || len(affected.TemplateCovered) != 0 {
			t.Errorf("paths %v: uncovered %v, template-covered %v", paths, affected.Uncovered, affected.TemplateCovered)
		}
		for _, match := range affected.Groups {
			switch match.Group.ID {
			case "verb-ratchet", "shipped-installation-standard", "agent-protocol-standard", "fast-static-build":
				continue
			}
			for _, input := range match.Group.Inputs {
				pattern, err := pathpattern.Parse(input)
				if err != nil {
					t.Fatal(err)
				}
				for _, path := range match.Paths {
					if pattern.Covers(path) {
						t.Errorf("extra group %s input %q covers %s", match.Group.ID, input, path)
					}
				}
			}
		}
	}
}

var _ = addLayoutCases(
	layoutCase{name: "help-test", help: objectHelpLayout("test")},
	layoutCase{name: "help-test-groups", help: actionHelpLayout("test groups")},
)
