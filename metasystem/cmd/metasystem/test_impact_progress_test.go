package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestImpactNamedGroupsEmitLandingPackageProgress(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	impactWrite(t, root, "go.mod", "module example.invalid/impact\n\ngo 1.27\n")
	impactWrite(t, root, "owner/owner_test.go", "package owner\nimport \"testing\"\nfunc TestSelected(t *testing.T) {t.Parallel()}\n")
	contract := testpolicy.Contract{SchemaVersion: 2, Groups: []testpolicy.Group{
		{ID: "canary", Adapter: "command", CWD: ".", Argv: []string{"/bin/sh", "-c", "exit 0"}, Format: "exit-status"},
		{ID: "unit/owner", Adapter: "go", CWD: ".", Packages: []string{"./owner"}, Tests: json.RawMessage(`["TestSelected"]`), Requires: []string{"canary"}},
	}}
	var out, problem bytes.Buffer
	code := runNamedTestGroups(root, contract, []string{"unit/owner"}, append(os.Environ(), "METASYSTEM_TESTING_WORKERS=9"), map[string]string{"unit/owner": "owner"}, &out, &problem)
	if code != 0 {
		t.Fatalf("exit=%d out=%s err=%s", code, &out, &problem)
	}
	text := out.String()
	for _, want := range []string{"landing planned 1\n", "landing package canary 0 ok ", "landing package owner 1 ok ", "LANDING-CHECKED\t0\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q: %s", want, text)
		}
	}
	if strings.Count(text, "landing planned 1\n") != 2 || strings.Index(text, "landing planned ") > strings.Index(text, "landing package ") {
		t.Fatalf("progress does not cover both proof groups: %s", text)
	}
}
