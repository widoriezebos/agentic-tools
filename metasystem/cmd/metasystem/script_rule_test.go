package main

// R11 (plans/designs/verbs-object-action.md 3.3, rules S1 and S2, VOA-24):
// no committed file under metasystem/ is a script except at a declared
// extension point, and nothing under metasystem/ depends on the VM
// definitions. Like the other ratchet witnesses these walk the filesystem and
// never call Git; the directories a checkout never commits (artifacts, bin,
// node_modules, nested checkouts) are skipped.

import (
	"os"
	"path"
	"strings"
	"testing"
)

// verbRatchetMetasystemScriptCeiling is the number of shell files under
// metasystem/ outside the extension-point allowlist. U9 takes it to zero by
// deleting the supervision-hook.sh stub.
const verbRatchetMetasystemScriptCeiling = 3

// scriptRuleSkipNames are the directories under metasystem/ that a checkout
// holds but never commits.
var scriptRuleSkipNames = []string{".git", "node_modules", "artifacts", "bin"}

// scriptRuleVMWord is the VM definitions' directory, assembled so this file
// does not name it literally.
var scriptRuleVMWord = strings.Join([]string{"environment", "vms"}, "/")

// scriptRuleIsScript reports whether a repository-relative path is a shell file.
func scriptRuleIsScript(rel string) bool {
	return strings.HasSuffix(rel, ".sh") || strings.HasSuffix(rel, ".bash")
}

// scriptRuleAllowed reports whether a repository-relative shell file sits at a
// declared extension point: a skill helper tool,
// metasystem/optional-skills/*/scripts/**.
func scriptRuleAllowed(rel string) bool {
	parts := strings.Split(rel, "/")
	return len(parts) >= 5 && parts[0] == "metasystem" && parts[1] == "optional-skills" &&
		parts[2] != "" && parts[3] == "scripts"
}

// scriptRuleExecutableOrConfig reports whether a repository-relative path is
// an executable or configuration file S2 judges: Go, shell, JSON, TOML, YAML
// and metasystem.conf*. Prose may name the VM.
func scriptRuleExecutableOrConfig(rel string) bool {
	base := path.Base(rel)
	if strings.HasPrefix(base, "metasystem.conf") {
		return true
	}
	switch path.Ext(base) {
	case ".go", ".sh", ".bash", ".json", ".toml", ".yaml", ".yml":
		return true
	}
	return false
}

// TestScriptRuleClassifiers pins the allowlist and the file classes with
// positive and negative cases.
func TestScriptRuleClassifiers(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		rel             string
		script, allowed bool
	}{
		{"metasystem/optional-skills/debug-java/scripts/preflight.sh", true, true},
		{"metasystem/optional-skills/x/scripts/deep/tool.bash", true, true},
		{"metasystem/optional-skills/x/preflight.sh", true, false},
		{"metasystem/optional-skills/scripts/tool.sh", true, false},
		{"metasystem/skills/x/scripts/tool.sh", true, false},
		{"metasystem/scripts/agents/supervision-hook.sh", true, false},
		{"metasystem/plans/first-headless-run/gate.sh", true, false},
		{"metasystem/scripts/agents/role-packets.json", false, false},
		{"metasystem/docs/tool.sh.md", false, false},
	} {
		if got := scriptRuleIsScript(row.rel); got != row.script {
			t.Errorf("scriptRuleIsScript(%q) = %v, want %v", row.rel, got, row.script)
		}
		if got := scriptRuleAllowed(row.rel); got != row.allowed {
			t.Errorf("scriptRuleAllowed(%q) = %v, want %v", row.rel, got, row.allowed)
		}
	}
	for _, row := range []struct {
		rel    string
		judged bool
	}{
		{"metasystem/internal/x/x.go", true},
		{"metasystem/testing.json", true},
		{"metasystem/metasystem.conf", true},
		{"metasystem/metasystem.conf.local", true},
		{"metasystem/scripts/enforcement/github-actions-metasystem.yml", true},
		{"metasystem/x.yaml", true},
		{"metasystem/x.toml", true},
		{"metasystem/x.bash", true},
		{"metasystem/docs/vm.md", false},
		{"metasystem/README.md", false},
		{"metasystem/x.txt", false},
	} {
		if got := scriptRuleExecutableOrConfig(row.rel); got != row.judged {
			t.Errorf("scriptRuleExecutableOrConfig(%q) = %v, want %v", row.rel, got, row.judged)
		}
	}
}

// TestVerbRatchetMetasystemScripts counts the shell files under metasystem/
// outside the extension-point allowlist (R11, rule S1).
func TestVerbRatchetMetasystemScripts(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	var sites []ratchetSite
	walkRatchetFiles(t, module, scriptRuleSkipNames, nil, func(_, rel string) {
		rel = "metasystem/" + rel
		if scriptRuleIsScript(rel) && !scriptRuleAllowed(rel) {
			sites = append(sites, ratchetSite{path: rel, line: 1, text: "a script outside the extension points"})
		}
	})
	checkVerbRatchet(t, "scripts under metasystem/ outside the extension points", "verbRatchetMetasystemScriptCeiling",
		len(sites), verbRatchetMetasystemScriptCeiling, sites)
}

// TestMetasystemNeverNamesTheVM fails on any reference to the VM definitions
// in an executable or configuration file under metasystem/ (R11, rule S2).
// This file is the witness and names the directory only in pieces.
func TestMetasystemNeverNamesTheVM(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	judged := 0
	var sites []ratchetSite
	walkRatchetFiles(t, module, scriptRuleSkipNames, nil, func(file, rel string) {
		rel = "metasystem/" + rel
		if !scriptRuleExecutableOrConfig(rel) {
			return
		}
		judged++
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, scriptRuleVMWord) {
				sites = append(sites, ratchetSite{path: rel, line: i + 1, text: strings.TrimSpace(line)})
			}
		}
	})
	if judged == 0 {
		t.Fatal("judged no executable or configuration file; the walk no longer reaches metasystem/")
	}
	if len(sites) > 0 {
		t.Errorf("metasystem/ names the VM definitions in %d executable or configuration lines (rule S2):\n%s", len(sites), ratchetSiteList(sites))
	}
}
