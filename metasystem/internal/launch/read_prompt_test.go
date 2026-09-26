package launch

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestReadPromptNamesItsDeclaredReport: a read whose launch declares one
// report file tells the reader that exact path in the prompt both runtime
// adapters send, since the reader has no other way to learn it; a read
// without a declared report and any other launch kind are unchanged.
func TestReadPromptNamesItsDeclaredReport(t *testing.T) {
	t.Parallel()
	outputs, _ := json.Marshal([]string{"/state/reads/r1/attempt-1/reports/read-1.md"})
	withReport := Record{Kind: "read", AdapterData: map[string]json.RawMessage{"declaredOutputs": outputs}}
	prompt := string(appendReadPacket([]byte("Examine the change.\n"), withReport))
	if !strings.Contains(prompt, "to exactly this file: /state/reads/r1/attempt-1/reports/read-1.md") {
		t.Fatalf("the prompt does not name the declared report:\n%s", prompt)
	}
	for _, record := range []Record{{Kind: "read"}, {Kind: "build", AdapterData: map[string]json.RawMessage{"declaredOutputs": outputs}}} {
		if prompt := string(appendReadPacket([]byte("brief\n"), record)); prompt != "brief\n" {
			t.Fatalf("%s launch prompt changed: %q", record.Kind, prompt)
		}
	}
}
