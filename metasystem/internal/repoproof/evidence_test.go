package repoproof

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func evidenceEvent(action, pkg, test, output string) string {
	data, _ := json.Marshal(struct{ Action, Package, Test, Output string }{action, pkg, test, output})
	return string(data) + "\n"
}

func TestTestEvidenceKeepsExactOutput(t *testing.T) {
	t.Parallel()
	unit := "metasystem/internal/example"
	pkg := "github.com/widoriezebos/agentic-tools/metasystem/internal/example"
	names := []string{"TestOne/sub/leaf", "TestEmpty"}
	first := strings.Repeat("α\tline\n", 20000)
	log := filepath.Join(t.TempDir(), "attempt.log")
	data := "{\n  \"section\": \"proof\"\n}\n" + evidenceEvent("output", pkg, names[0], first) + evidenceEvent("output", pkg, "TestOther", "unrelated output") + evidenceEvent("output", pkg, names[0], "last\r\n") +
		evidenceEvent("fail", pkg, names[0], "") + evidenceEvent("fail", pkg, names[1], "") + evidenceEvent("fail", pkg, "", "") + evidenceEvent("pass", pkg, names[0], "") + evidenceEvent("pass", pkg, "", "") + "LANDING-FAILED\t" + unit + "\t" + strings.Join(names, " ") + "\nLANDING-CHECKED\t1\n"
	if err := os.WriteFile(log, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	for _, pass := range []string{"first", "replay"} {
		outputs, err := TestEvidence(log, unit, names, "fail")
		if err != nil {
			t.Fatal(err)
		}
		for i, name := range names {
			want := ""
			if i == 0 {
				want = first + "last\r\n"
			}
			output := outputs[name]
			got, err := os.ReadFile(output.Path)
			if err != nil || string(got) != want || output.Digest != fmt.Sprintf("%x", sha256.Sum256([]byte(want))) || output.Outcome != "fail" {
				t.Fatalf("%s %s: output=%+v bytes=%d error=%v", pass, name, output, len(got), err)
			}
		}
	}
	// Evidence files are immutable even when a retained log is changed.
	if err := os.WriteFile(log, []byte(strings.Replace(data, "last\\r\\n", "changed\\r\\n", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := TestEvidence(log, unit, names, "fail"); err == nil {
		t.Fatal("changed output overwrote immutable evidence")
	}
}

func TestTestEvidenceRefusesUnavailableOutput(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"summary", "unfinished package", "missing test", "wrong outcome", "malformed", "read error", "write error", "unreported failure"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			log := filepath.Join(t.TempDir(), "attempt.log")
			test := "TestOne"
			data := evidenceEvent("fail", "unit", test, "") + evidenceEvent("fail", "unit", "", "")
			switch name {
			case "summary":
				data = "LANDING-FAILED\tunit\tTestOne\nLANDING-CHECKED\t1\n"
			case "unfinished package":
				data = evidenceEvent("fail", "unit", test, "")
			case "missing test":
				data = evidenceEvent("fail", "unit", "Other", "") + evidenceEvent("fail", "unit", "", "")
			case "wrong outcome":
				data = evidenceEvent("pass", "unit", test, "") + evidenceEvent("pass", "unit", "", "")
			case "unreported failure":
				data = evidenceEvent("fail", "unit", "Omitted", "") + data
			case "malformed":
				data = "{broken\n" + data
			}
			if name != "read error" {
				if err := os.WriteFile(log, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if name == "write error" {
				key, _ := json.Marshal([2]string{"unit", test})
				if err := os.Mkdir(fmt.Sprintf("%s.%x.output", log, sha256.Sum256(key)), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := TestEvidence(log, "unit", []string{test}, "fail"); err == nil {
				t.Fatalf("%s authorized missing evidence", name)
			}
		})
	}
}
