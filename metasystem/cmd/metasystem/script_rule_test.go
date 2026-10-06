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
// metasystem/ outside the extension points. It is zero since U9 deleted the
// supervision-hook.sh stub; it never rises.
const verbRatchetMetasystemScriptCeiling = 0

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
// declared extension point: a skill helper tool or the repository's committed
// full-proof launcher. Proof decisions and reporting stay in Go.
func scriptRuleAllowed(rel string) bool {
	if rel == "metasystem/proof/full.sh" {
		return true
	}
	parts := strings.Split(rel, "/")
	return len(parts) >= 5 && parts[0] == "metasystem" && parts[1] == "optional-skills" &&
		parts[2] != "" && parts[3] == "scripts"
}

// scriptRuleContractInstruments returns the repository-relative scripts a
// mission contract runs as its gate or a guard: the shell-file words of every
// gate.command= and guard.<name>.command= line inside its ```mission block,
// relative to metasystem/ where the mission runner runs them. Those are the
// mission's authored instruments, an extension point like a skill's helper
// tools (the birth-token mission runs plans/first-headless-run/gate.sh and
// guard.sh), so S1 does not count them while a contract names them.
func scriptRuleContractInstruments(text string) []string {
	var out []string
	inMission := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inMission = !inMission && strings.TrimSpace(strings.TrimPrefix(trimmed, "```")) == "mission"
			continue
		}
		if !inMission {
			continue
		}
		key, value, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		parts := strings.Split(key, ".")
		isGate := key == "gate.command"
		isGuard := len(parts) == 3 && parts[0] == "guard" && parts[1] != "" && parts[2] == "command"
		if !isGate && !isGuard {
			continue
		}
		for _, word := range strings.Fields(value) {
			if !scriptRuleIsScript(word) || path.IsAbs(word) {
				continue
			}
			clean := path.Clean(word)
			if clean == ".." || strings.HasPrefix(clean, "../") {
				continue
			}
			out = append(out, "metasystem/"+clean)
		}
	}
	return out
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
	// The repository's full proof uses one plumbing launcher; other scripts
	// in its directory remain forbidden.
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
		{"metasystem/proof/full.sh", true, true},
		{"metasystem/proof/other.sh", true, false},
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
	contract := strings.Join([]string{
		"gate.command=bash plans/outside-the-block.sh",
		"```mission",
		"gate.command=bash plans/first-headless-run/gate.sh",
		"gate.paths=plans/first-headless-run/*.sh",
		"guard.build.command=bash plans/first-headless-run/guard.sh",
		"guard.build.floor=1",
		"guard.x.y.command=bash plans/three-part-name.sh",
		"guard.lint.command=bash /abs/lint.sh ../escape.sh ./plans/lint.bash",
		"truth.command=bash plans/truth.sh",
		"```",
		"guard.late.command=bash plans/after-the-block.sh",
	}, "\n")
	got := strings.Join(scriptRuleContractInstruments(contract), " ")
	want := "metasystem/plans/first-headless-run/gate.sh metasystem/plans/first-headless-run/guard.sh metasystem/plans/lint.bash"
	if got != want {
		t.Errorf("scriptRuleContractInstruments = %q, want %q", got, want)
	}
}

// scriptRuleMissionInstruments collects the instruments every mission
// contract under metasystem/ names (files ending .contract.md).
func scriptRuleMissionInstruments(t *testing.T, module string) map[string]bool {
	t.Helper()
	named := map[string]bool{}
	walkRatchetFiles(t, module, scriptRuleSkipNames, nil, func(file, rel string) {
		if !strings.HasSuffix(rel, ".contract.md") {
			return
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, script := range scriptRuleContractInstruments(string(data)) {
			named[script] = true
		}
	})
	return named
}

// TestVerbRatchetMetasystemScripts counts the shell files under metasystem/
// outside the extension points (R11, rule S1): the skill helper tools, and
// the gate and guard instruments a mission contract names.
func TestVerbRatchetMetasystemScripts(t *testing.T) {
	t.Parallel()
	_, module := verbRatchetRoots(t)
	instruments := scriptRuleMissionInstruments(t, module)
	var sites []ratchetSite
	walkRatchetFiles(t, module, scriptRuleSkipNames, nil, func(_, rel string) {
		rel = "metasystem/" + rel
		if scriptRuleIsScript(rel) && !scriptRuleAllowed(rel) && !instruments[rel] {
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
