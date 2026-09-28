package external

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// A new runtime whose describe is malformed is refused with a reason that
// names the defect, and the registry never lists it as an accepted runtime.
func TestLoadRefusesMalformedDescribe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cases := map[string]struct{ describe, want string }{
		"wrongname":  {`{"schemaVersion":1,"name":"other","match":["^wrongname"],"positive":"wrongname","lookalike":"x"}`, `describes itself as "other"`},
		"nomatch":    {`{"schemaVersion":1,"name":"nomatch","match":[],"positive":"nomatch","lookalike":"x"}`, "declares no match pattern"},
		"blankpat":   {`{"schemaVersion":1,"name":"blankpat","match":[" ^blankpat"],"positive":"blankpat","lookalike":"x"}`, "malformed pattern"},
		"badregex":   {`{"schemaVersion":1,"name":"badregex","match":["^(badregex"],"positive":"badregex","lookalike":"x"}`, `pattern "^(badregex"`},
		"badenforce": {`{"schemaVersion":1,"name":"badenforce","match":["^badenforce"],"positive":"badenforce","lookalike":"x","enforcement":{"network":"sometimes"}}`, `enforcement network="sometimes" is neither mapped nor notEnforced`},
		"oldschema":  {`{"schemaVersion":2,"name":"oldschema","match":["^oldschema"]}`, "schemaVersion 2, want 1"},
		"notjson":    {`not json`, "response is not JSON"},
		"novectors":  {`{"schemaVersion":1,"name":"novectors","match":["^novectors"]}`, "must declare a positive and a lookalike vector"},
	}
	for name, c := range cases {
		install(t, root, name, describeScript(c.describe), 0o755, true)
	}
	// An adapter whose describe exits non-zero (not 64) fails with its stderr.
	install(t, root, "crashes", "#!/bin/sh\necho boom >&2\nexit 3\n", 0o755, true)
	cases["crashes"] = struct{ describe, want string }{"", "external adapter crashes describe failed"}
	// A new runtime may not delegate describe to a built-in it does not have.
	install(t, root, "delegates", "#!/bin/sh\nexit 64\n", 0o755, true)
	cases["delegates"] = struct{ describe, want string }{"", "describe is required of a new runtime"}

	reg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, c := range cases {
		refusal, ok := reg.Refusal(name)
		if !ok || !strings.Contains(refusal.Reason, c.want) {
			t.Fatalf("%s: refusal = %+v (found %v), want reason containing %q", name, refusal, ok, c.want)
		}
		if !strings.Contains(refusal.Error(), "external adapter "+name+" (") || !strings.Contains(refusal.Error(), "is refused: ") {
			t.Fatalf("%s: Error() = %q", name, refusal.Error())
		}
		if entry, ok := reg.Lookup(name); ok && !entry.Builtin && entry.Refused == nil {
			t.Fatalf("%s: a refused adapter is listed as accepted: %+v", name, entry)
		}
	}
	if _, ok := reg.Refusal("nosuch"); ok {
		t.Fatal("Refusal found a name that was never refused")
	}
}

// Report prints one line per accepted external runtime and override and one
// per refusal; Externals lists only executable-backed entries.
func TestReportAndExternals(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	newagent := install(t, root, "newagent", describeScript(newagentDescribe), 0o755, true)
	fake := install(t, root, "fake", "#!/bin/sh\nexit 64\n", 0o755, true)
	unnamed := install(t, root, "unnamed", describeScript(newagentDescribe), 0o755, false)
	reg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range reg.Externals() {
		names = append(names, e.Name)
		if e.Adapter == nil {
			t.Fatalf("Externals returned %s without an executable", e.Name)
		}
	}
	if strings.Join(names, " ") != "fake newagent" {
		t.Fatalf("Externals = %v", names)
	}
	if e, _ := reg.Lookup("newagent"); !e.External() || e.Overridden() {
		t.Fatalf("newagent: External=%v Overridden=%v", e.External(), e.Overridden())
	}
	if e, _ := reg.Lookup("fake"); e.External() || !e.Overridden() {
		t.Fatalf("fake: External=%v Overridden=%v", e.External(), e.Overridden())
	}
	var out bytes.Buffer
	reg.Report(&out)
	want := "adapter fake: built-in overridden by " + fake + "\n" +
		"adapter newagent: external runtime " + newagent + "\n" +
		"adapter unnamed: refused (" + unnamed + "): " + reg.Refusals[0].Reason + "\n"
	if out.String() != want {
		t.Fatalf("Report =\n%s\nwant\n%s", out.String(), want)
	}
}

// A refused override is listed among Externals but Report prints it only as
// a refusal, never as an accepted override.
func TestReportSkipsRefusedOverride(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	install(t, root, "fake", "#!/bin/sh\necho broken >&2\nexit 5\n", 0o755, true)
	reg, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	entry, _ := reg.Lookup("fake")
	if entry.Refused == nil || entry.Overridden() {
		t.Fatalf("fake entry = %+v", entry)
	}
	var out bytes.Buffer
	reg.Report(&out)
	if strings.Contains(out.String(), "built-in overridden") || !strings.Contains(out.String(), "adapter fake: refused (") {
		t.Fatalf("Report = %q", out.String())
	}
}

// Call hands the request on stdin with schema fields added, maps exit 64 to
// ErrDelegated, and names the operation and stderr on any other failure.
func TestCallContract(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := install(t, root, "echoer", "#!/bin/sh\ncase \"$1\" in\n  probe) cat ;;\n  cancel) exit 64 ;;\n  *) echo \"no $1\" >&2; exit 2 ;;\nesac\n", 0o755, true)
	a := Adapter{Name: "echoer", Path: path}
	got, err := a.Call("probe", map[string]any{"k": "v"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"schemaVersion":1`, `"operation":"probe"`, `"runtime":"echoer"`, `"k":"v"`} {
		if !strings.Contains(string(got), want) {
			t.Fatalf("stdin echo %q lacks %s", got, want)
		}
	}
	var decoded struct{ K string }
	if err := Decode(got, &decoded); err != nil || decoded.K != "v" {
		t.Fatalf("Decode = %+v, %v", decoded, err)
	}
	if _, err := a.Call("cancel", nil, nil); !errors.Is(err, ErrDelegated) {
		t.Fatalf("exit 64 err = %v", err)
	}
	_, err = a.Call("repair", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "external adapter echoer repair failed") || !strings.Contains(err.Error(), "no repair") {
		t.Fatalf("failure err = %v", err)
	}
	if _, err := a.Call("probe", map[string]any{"bad": func() {}}, nil); err == nil {
		t.Fatal("an unencodable request must fail")
	}
}
