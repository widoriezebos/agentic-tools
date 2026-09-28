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
// 3.5): the Devin ancestry tests in classify_test.go also stage their
// ancestry under registry roots — an external runtime whose process
// classifies DELEGATE through its describe, and overrides of Devin, where
// keeping the reserved `devin acp` exclusion still lets a host tool call walk
// through the helper to its announced main (VOA-31). The registry's universe
// is used as is: no signature stub, only the fixture root's narrowing to the
// staged runtimes.

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
	// The installation's registry, not a pinned universe (an earlier stage
	// of the same test may have pinned one).
	pinned := delegateSignatures
	delegateSignatures = func(root string) ([]census.Signature, error) {
		sigs, _, _, err := census.InstalledAdapterSignatures(root)
		return sigs, err
	}
	got, err := Classify(root, child)
	delegateSignatures = pinned
	if err != nil {
		t.Fatal(err)
	}
	return got
}
