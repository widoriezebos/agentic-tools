package audit

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestShellCommandWordsDistinguishesCommandsFromData(t *testing.T) {
	source := strings.Join([]string{
		`node -v`,
		`  node -v`,
		`true && node -v`,
		`value=$(node -v)`,
		"value=`node -v`",
		`if node -v; then :; fi`,
		`env MODE=test node -v`,
		`command node -v`,
		`exec node -v`,
		`node=x; node -v`,
		`printf "%s\\n" node`,
		`echo "Let a node finish it."; node -v`,
		`intent='Wait for the first node.'`,
		`node=x`,
		`echo "$(node -v)"`,
		`echo before # node -v; node -v`,
		`echo "# prose"; /usr/bin/node -v`,
		`value=$((node + 1))`,
		`printf '%s' foo\\ bar`,
	}, "\n")
	commands := shellCommandWords(source)
	var nodes int
	for _, command := range commands {
		if filepath.Base(command) == "node" {
			nodes++
		}
	}
	if nodes != 13 {
		t.Fatalf("node command count = %d, want 13; commands=%q", nodes, commands)
	}
	for _, command := range commands {
		if strings.Contains(command, "Wait for") || command == "node=x" {
			t.Fatalf("data became a command: %q in %q", command, commands)
		}
	}
}

func TestShellCommandWordsCoversShellLexicalEdges(t *testing.T) {
	commands := shellCommandWords(strings.Join([]string{
		"\t { node; } | (perl) & ruby",
		"env -i node -v",
		"echo foo#bar; $runner; echo trailing\\",
		"echo \"pre `php -v` post\"; deno -V",
		"echo \"escaped \\\"node\\\" data\"",
		"value=$(((node + 1))); printf done",
		"echo \"$((node + 1))\"",
		") node -v",
	}, "\n"))
	_ = shellCommandWords("echo 'unterminated quoted node")
	_ = shellCommandWords("echo \"unterminated quoted node")
	joined := strings.Join(commands, ",")
	for _, command := range []string{"node", "perl", "ruby", "php", "deno", "$runner"} {
		if !strings.Contains(","+joined+",", ","+command+",") {
			t.Errorf("missing command %q in %q", command, commands)
		}
	}
	if strings.Count(joined, "node") != 3 {
		t.Fatalf("quoted or arithmetic node data became executable: %q", commands)
	}
}

func TestShellCommandWordsFindsCommandsNestedInArithmetic(t *testing.T) {
	commands := shellCommandWords(strings.Join([]string{
		`value=$(( \`,
		`  $((node + 1)) +`,
		`  $(node -v) +`,
		"  `node -v` +",
		`  "$(node -v)" +`,
		`  (node + 1)`,
		`))`,
	}, "\n"))
	var nodes int
	for _, command := range commands {
		if filepath.Base(command) == "node" {
			nodes++
		}
	}
	if nodes != 3 {
		t.Fatalf("arithmetic nested command count = %d, want 3; commands=%q", nodes, commands)
	}
}

func TestAuditDependenciesReportsEveryForbiddenCommandAndPythonDebt(t *testing.T) {
	root := t.TempDir()
	writeDependencyTestFile(t, filepath.Join(root, "scripts", "agents", "bad.sh"), strings.Join([]string{
		"#!/usr/bin/env bash",
		"node -v",
		"printf '%s' okay; perl -e 1",
		"python3 -c pass",
	}, "\n"))
	writeDependencyTestFile(t, filepath.Join(root, "scripts", "agents", "safe.sh"), "echo 'node is prose'; printf '%s' node\n")
	writeDependencyTestFile(t, filepath.Join(root, "scripts", "agents", "dependency-ratchet.sh"), "ruby -v\n")
	writeDependencyTestFile(t, filepath.Join(root, "scripts", "agents", "channel-fixtures.sh"), "python3 -c pass\n")

	findings, err := AuditDependencies(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 3 {
		t.Fatalf("findings = %#v, want three", findings)
	}
	got := []string{findings[0].String(), findings[1].String(), findings[2].String()}
	for index, want := range []string{
		"banned interpreter node: scripts/agents/bad.sh:2",
		"banned interpreter perl: scripts/agents/bad.sh:3",
		"python3 outside the declared legacy sites python3: scripts/agents/bad.sh:4",
	} {
		if got[index] != want {
			t.Fatalf("finding %d = %q, want %q", index, got[index], want)
		}
	}
}

func TestAuditDependenciesPreservesLogicalContextAndPhysicalLines(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		wantLines []int
	}{
		{"quoted arithmetic variable is inert", "echo \"$((node + 1))\"\n", nil},
		{"unquoted arithmetic variable is inert", "value=$((node + 1))\n", nil},
		{"nested command in arithmetic", "value=$(( $(node -v) + 1 ))\n", []int{1}},
		{"env unset option operand", "env -u NAME node -v\n", []int{1}},
		{"exec name option operand", "exec -a NAME node -v\n", []int{1}},
		{"continued printf argument is inert", "printf '%s\\n' \\\n  node\n", nil},
		{"ordinary command substitution", "value=$(node -v)\n", []int{1}},
		{"plain executable physical line", "# inert first line\nprintf '%s\\n' node\nnode -v\n", []int{3}},
		{"continued env executable physical line", "env \\\n  node -v\n", []int{2}},
		{"continued double quoted data is inert", "printf \"%s \\\nnode\"\n", nil},
		{"multiline single quoted data is inert", "printf '%s\nnode\n'\n", nil},
		{"env option operand named node is inert", "env -u node printf ok\n", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeDependencyTestFile(t, filepath.Join(root, "scripts", "context.sh"), test.source)
			findings, err := AuditDependencies(root)
			if err != nil {
				t.Fatal(err)
			}
			var gotLines []int
			for _, finding := range findings {
				if finding.Interpreter != "node" || finding.Path != "scripts/context.sh" || finding.PythonDebt {
					t.Errorf("unexpected finding: %#v", finding)
				}
				gotLines = append(gotLines, finding.Line)
			}
			if !reflect.DeepEqual(gotLines, test.wantLines) {
				t.Fatalf("executable node lines = %v, want %v", gotLines, test.wantLines)
			}
		})
	}
}

func TestAuditDependenciesRejectsMissingOrNonDirectoryScriptsRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := AuditDependencies(root); err == nil || !strings.Contains(err.Error(), "scripts root unreadable") {
		t.Fatalf("missing scripts root error = %v", err)
	}
	writeDependencyTestFile(t, filepath.Join(root, "scripts"), "not a directory")
	if _, err := AuditDependencies(root); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("file scripts root error = %v", err)
	}
}

func TestAuditDependenciesReportsScanFailures(t *testing.T) {
	brokenRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(brokenRoot, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(brokenRoot, "missing"), filepath.Join(brokenRoot, "scripts", "broken.sh")); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditDependencies(brokenRoot); err == nil || !strings.Contains(err.Error(), "scan failed") {
		t.Fatalf("broken source scan error = %v", err)
	}
}

func TestAuditDependenciesSortsAcrossPathsLinesAndInterpreters(t *testing.T) {
	root := t.TempDir()
	writeDependencyTestFile(t, filepath.Join(root, "scripts", "z.sh"), "ruby -v; node -v\n")
	writeDependencyTestFile(t, filepath.Join(root, "scripts", "a.sh"), "\nphp -v\n")
	findings, err := AuditDependencies(root)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(findings))
	for _, finding := range findings {
		got = append(got, finding.String())
	}
	want := []string{
		"banned interpreter php: scripts/a.sh:2",
		"banned interpreter node: scripts/z.sh:1",
		"banned interpreter ruby: scripts/z.sh:1",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("sorted findings = %q, want %q", got, want)
	}
}

func TestShellAssignmentValidation(t *testing.T) {
	for value, want := range map[string]bool{"NAME=value": true, "_N2=value": true, "2BAD=value": false, "noequals": false, "=empty": false} {
		if got := shellAssignment(value); got != want {
			t.Errorf("shellAssignment(%q) = %v, want %v", value, got, want)
		}
	}
	for _, test := range []struct {
		prefix string
		option string
		want   bool
	}{{"env", "-u", true}, {"exec", "-a", true}, {"env", "-i", false}, {"command", "-v", false}} {
		if got := shellPrefixOptionTakesOperand(test.prefix, test.option); got != test.want {
			t.Errorf("shellPrefixOptionTakesOperand(%q, %q) = %v, want %v", test.prefix, test.option, got, test.want)
		}
	}
}

func writeDependencyTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
