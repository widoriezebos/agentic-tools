package external

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// describeScript is an adapter whose describe answers the given JSON and
// whose every other operation exits 64.
func describeScript(describe string) string {
	return fmt.Sprintf("#!/bin/sh\ncase \"$1\" in\n  describe) printf '%%s\\n' '%s' ;;\n  *) exit 64 ;;\nesac\n", describe)
}

const newagentDescribe = `{"schemaVersion":1,"name":"newagent","match":["^([^[:space:]]*/)?newagent([[:space:]]|$)"],"exclude":["newagent-helper"],"positive":"newagent -p task","lookalike":"newagent-helper serve"}`

func install(t *testing.T, root, name, body string, mode os.FileMode, use bool) string {
	t.Helper()
	path := filepath.Join(root, Dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	if use {
		conf := filepath.Join(root, "metasystem.conf")
		existing, _ := os.ReadFile(conf)
		if err := os.WriteFile(conf, append(existing, []byte(UseKey(name)+"="+UseValue+"\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// TestDiscoveryTrustRules: an adapter runs only when named in the
// configuration and safe on disk; a built-in's name is an override; every
// other file is reported with its fix, never executed.
func TestDiscoveryTrustRules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	install(t, root, "newagent", describeScript(newagentDescribe), 0o755, true)
	install(t, root, "unnamed", describeScript(newagentDescribe), 0o755, false)
	loose := install(t, root, "loose", describeScript(newagentDescribe), 0o775, true)
	install(t, root, "fake", describeScript(newagentDescribe), 0o755, true)
	install(t, root, "Bad_Name", describeScript(newagentDescribe), 0o755, true)
	adapters, refusals, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, a := range adapters {
		names = append(names, a.Name+map[bool]string{true: "(override)", false: ""}[a.Overrides])
	}
	if strings.Join(names, " ") != "fake(override) newagent" {
		t.Fatalf("adapters = %v", names)
	}
	reasons := map[string]string{}
	for _, r := range refusals {
		reasons[r.Name] = r.Reason
	}
	if !strings.Contains(reasons["unnamed"], "add adapters.unnamed.use=external") ||
		!strings.Contains(reasons["loose"], "fix it with: chmod go-w "+loose) || !strings.Contains(reasons["Bad_Name"], "rename the file") {
		t.Fatalf("refusals = %v", reasons)
	}
	if _, _, err := Lookup(root, "unnamed"); err == nil {
		t.Fatal("an unnamed adapter resolved")
	}
}

// TestSignatureFromDescribe: the registry reads an external runtime's
// classification signature from describe and adds the shared exclusions;
// exit 64 is the delegation.
func TestSignatureFromDescribe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	install(t, root, "newagent", describeScript(newagentDescribe), 0o755, true)
	adapter, found, err := Lookup(root, "newagent")
	if err != nil || !found {
		t.Fatalf("lookup = %v %v", found, err)
	}
	text, err := adapter.SignatureText()
	if err != nil || text != "match ^([^[:space:]]*/)?newagent([[:space:]]|$)\nexclude newagent-helper\n" {
		t.Fatalf("signature = %q %v", text, err)
	}
	if _, err := adapter.Call("prepare", nil, nil); !errors.Is(err, ErrDelegated) {
		t.Fatalf("exit 64 = %v, want ErrDelegated", err)
	}
	reg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := reg.Lookup("newagent")
	if !ok || entry.Builtin || entry.Adapter == nil || !strings.Contains(entry.Signature, "exclude supervision-hook") {
		t.Fatalf("entry = %+v", entry)
	}
}

// TestRegistryCrossCheck: a declaration that would claim another runtime's
// processes is refused with the reason, and so is one whose own positive
// vector another runtime already claims (VOA-28).
func TestRegistryCrossCheck(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, describe, reason string
	}{
		{"greedy", `{"schemaVersion":1,"name":"greedy","match":["(codex|greedy)"],"positive":"greedy run","lookalike":"x-helper"}`, `claims "codex", a process of runtime codex`},
		{"acpish", `{"schemaVersion":1,"name":"acpish","match":["(^|/)(devin acp|acpish)"],"positive":"acpish run","lookalike":"x-helper"}`, `claims "devin acp", a process of runtime devin`},
		{"shadow", `{"schemaVersion":1,"name":"shadow","match":["^shadow-x"],"positive":"claude","lookalike":"x-helper"}`, "does not classify its own positive vector"},
		{"hidden", `{"schemaVersion":1,"name":"hidden","match":["^hidden","^codex exec$"],"positive":"codex exec","lookalike":"x"}`, `runtime codex's signature already claims its positive vector`},
		{"novectors", `{"schemaVersion":1,"name":"novectors","match":["^novectors"]}`, "positive and a lookalike vector"},
		{"misnamed", `{"schemaVersion":1,"name":"other","match":["^misnamed"],"positive":"misnamed","lookalike":"x"}`, "the name must equal the file name"},
	} {
		root := t.TempDir()
		install(t, root, test.name, describeScript(test.describe), 0o755, true)
		reg, err := Load(root)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := reg.Lookup(test.name); ok {
			t.Errorf("%s: admitted", test.name)
			continue
		}
		refusal, refused := reg.Refusal(test.name)
		if !refused || !strings.Contains(refusal.Reason, test.reason) {
			t.Errorf("%s: refusal = %+v, want %q", test.name, refusal, test.reason)
		}
	}
}

// TestOverrideKeepsReservedExclusions is VOA-31: an override of Devin whose
// describe drops the `devin acp` exclusion is refused (the built-in's
// declaration stays in force for recognition, and running it names the
// fix); one that keeps it becomes the one effective declaration, carrying
// the built-in's exclusions. Matching Devin's own positive vector is not a
// conflict.
func TestOverrideKeepsReservedExclusions(t *testing.T) {
	t.Parallel()
	dropped := t.TempDir()
	install(t, dropped, "devin", describeScript(`{"schemaVersion":1,"name":"devin","match":["^([^[:space:]]*/)?devin([[:space:]]|$)"],"lookalike":"metasystem-devin-lookalike"}`), 0o755, true)
	reg, err := Load(dropped)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := reg.Lookup("devin")
	if !ok || entry.Refused == nil || !strings.Contains(entry.Refused.Reason, `claims "devin acp"`) || !strings.Contains(entry.Refused.Reason, "keep the built-in's exclusion") {
		t.Fatalf("dropped exclusion entry = %+v", entry)
	}
	if !strings.Contains(entry.Signature, "exclude ^([^[:space:]]*/)?devin[[:space:]]+acp") {
		t.Fatalf("a refused override changed the recognizers' signature:\n%s", entry.Signature)
	}

	kept := t.TempDir()
	install(t, kept, "devin", describeScript(`{"schemaVersion":1,"name":"devin","match":["^([^[:space:]]*/)?devin([[:space:]]|$)"],"exclude":["^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)"]}`), 0o755, true)
	reg, err = Load(kept)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok = reg.Lookup("devin")
	if !ok || entry.Refused != nil || !entry.Overridden() {
		t.Fatalf("kept exclusion entry = %+v, refusals %v", entry, reg.Refusals)
	}
	for _, line := range []string{"exclude supervision-hook", "delegate-supervisor"} {
		if !strings.Contains(entry.Signature, line) {
			t.Fatalf("the effective declaration lost the built-in's exclusion %q:\n%s", line, entry.Signature)
		}
	}
	if strings.Contains(entry.Signature, "devin-delegate-acp") {
		t.Fatalf("the override's own match lines are not the effective ones:\n%s", entry.Signature)
	}

	delegated := t.TempDir()
	install(t, delegated, "devin", "#!/bin/sh\nexit 64\n", 0o755, true)
	reg, err = Load(delegated)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok = reg.Lookup("devin")
	if !ok || !entry.Overridden() || !strings.Contains(entry.Signature, "devin-delegate-acp") {
		t.Fatalf("an override that leaves describe to the built-in = %+v", entry)
	}
}

// TestDescribeIsMemoizedPerFile: the recognizers ask describe once per
// process for an unchanged executable, and again after it changes.
func TestDescribeIsMemoizedPerFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	counter := filepath.Join(root, "describes")
	body := "#!/bin/sh\nprintf x >>'" + counter + "'\nprintf '%s\\n' '" + newagentDescribe + "'\n"
	install(t, root, "newagent", body, 0o755, true)
	for range 3 {
		if _, err := Load(root); err != nil {
			t.Fatal(err)
		}
	}
	if data, _ := os.ReadFile(counter); string(data) != "x" {
		t.Fatalf("describe ran %q times, want once", data)
	}
	if err := testexec.WriteFile(filepath.Join(root, Dir, "newagent"), []byte(body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(counter); string(data) != "xx" {
		t.Fatalf("a changed executable was not described again: %q", data)
	}
}
