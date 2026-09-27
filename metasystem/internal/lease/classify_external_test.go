package lease

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
)

// Lease classification on the runtime registry (design verbs-object-action
// 3.5): an external runtime's process classifies DELEGATE through its
// describe, and an override of Devin that keeps the reserved `devin acp`
// exclusion still lets a host tool call walk through the helper to its
// announced main (VOA-31). The registry's universe is used as is: no
// signature stub, only the fixture root's narrowing to the staged runtimes.

const (
	newagentDescribe    = `{"schemaVersion":1,"name":"newagent","match":["^([^[:space:]]*/)?newagent([[:space:]]|$)"],"positive":"newagent -p task","lookalike":"newagent-helper serve"}`
	devinKeepsHelper    = `{"schemaVersion":1,"name":"devin","match":["^([^[:space:]]*/)?devin([[:space:]]|$)","^([^[:space:]]*/)?devin-delegate-acp([[:space:]]|$)"],"exclude":["^([^[:space:]]*/)?devin[[:space:]]+acp([[:space:]]|$)"]}`
	devinDropsTheHelper = `{"schemaVersion":1,"name":"devin","match":["^([^[:space:]]*/)?devin([[:space:]]|$)"]}`
)

// classifyUnderAdapters stages announced main (this test) <- intermediate
// with the given fixture command <- tool child, in a root whose registry
// holds the given adapters, and classifies the child.
func classifyUnderAdapters(t *testing.T, intermediateCommand string, adapters map[string]string) Classification {
	t.Helper()
	root := t.TempDir()
	conf := "metasystem.runtimes=fake\n"
	for name, describe := range adapters {
		path := filepath.Join(root, "adapters", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "#!/bin/sh\n[ \"$1\" = describe ] || exit 64\nprintf '%s\\n' '" + describe + "'\n"
		if err := testexec.WriteFile(path, []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
		conf += "adapters." + name + ".use=external\n"
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv(census.FixtureSignatureRuntimesEnv, "devin,fake,newagent")
	self := int64(os.Getpid())
	if _, err := Announce(root, "sess", self, selfStart(t), "tag", "fake", ""); err != nil {
		t.Fatalf("announce self: %v", err)
	}
	intermediate, child := grandchild(t)
	tablePath := filepath.Join(root, "identity.json")
	if err := os.WriteFile(tablePath, []byte(fmt.Sprintf(`{"%d":{"pidStartedAt":1,"command":%q}}`, intermediate, intermediateCommand)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", tablePath)
	got, err := Classify(root, child)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestClassifyExternalRuntimeProcessIsDelegate(t *testing.T) {
	got := classifyUnderAdapters(t, "/opt/newagent/bin/newagent -p task --tag t", map[string]string{"newagent": newagentDescribe})
	if got.Class != ClassDelegate {
		t.Fatalf("a tool shell under an external runtime's CLI must classify DELEGATE; got %+v", got)
	}
}

func TestClassifyDevinOverrideKeepsTheHostHelperTransparent(t *testing.T) {
	for name, describe := range map[string]string{"kept": devinKeepsHelper, "dropped (refused)": devinDropsTheHelper} {
		if got := classifyUnderAdapters(t, "devin acp", map[string]string{"devin": describe}); got.Class != ClassMain {
			t.Fatalf("%s: a host tool call under devin acp must classify MAIN through the reserved exclusion; got %+v", name, got)
		}
	}
	if got := classifyUnderAdapters(t, "devin-delegate-acp acp", map[string]string{"devin": devinKeepsHelper}); got.Class != ClassDelegate {
		t.Fatalf("the delegate ACP server must stay DELEGATE under the override; got %+v", got)
	}
}
