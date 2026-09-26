package lifecycle

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Whether this server can act as the human is a fact of the run it was
// started in, and `ui status` runs in a different process entirely. So the
// server writes the outcome into its record at boot, and status reads it back
// and says it — as evidence, never as a credential.

func TestStatusReportsWhatTheBootProofFound(t *testing.T) {
	t.Parallel()

	for name, line := range map[string]string{
		"proven":     "acting as human:Wido — proven at the enrolled terminal",
		"not proven": "not proven: the interface was started by an agent process (claude-code); start it from your own terminal with bin/metasystem ui restart to act as yourself",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stateRoot, prober := authorityFixture(t, line)

			result := StatusResult(stateRoot, prober, func() (string, error) { return "sha256:serving", nil })

			testutil.Expect(t, "the lines", len(result.Lines), 2)
			testutil.Expect(t, "the authority line", result.Lines[1], line)
			testutil.Expect(t, "the code", result.Code, 0)
		})
	}
}

// A record written before this build carries no outcome. Status says nothing
// about authority rather than guessing that there was none.
func TestStatusSaysNothingAboutAnUnrecordedAuthority(t *testing.T) {
	t.Parallel()

	stateRoot, prober := authorityFixture(t, "")

	result := StatusResult(stateRoot, prober, func() (string, error) { return "sha256:serving", nil })

	testutil.Expect(t, "the lines", len(result.Lines), 1)
}

// The outcome rides in the record the server writes, under a key an older
// reader ignores, which is why the schema stays 1.
func TestServeWritesTheAuthorityOutcomeIntoTheRecord(t *testing.T) {
	t.Parallel()

	stateRoot, _ := authorityFixture(t, "acting as human:Wido — proven at the enrolled terminal")

	data, err := os.ReadFile(recordPath(stateRoot))
	testutil.Require(t, "read the record", err, nil)
	var raw map[string]any
	testutil.Require(t, "decode the record", json.Unmarshal(data, &raw), nil)
	testutil.Expect(t, "the schema is unchanged", raw["schemaVersion"], float64(1))
	testutil.Expect(t, "the outcome", raw["authority"], "acting as human:Wido — proven at the enrolled terminal")
}

func authorityFixture(t *testing.T, line string) (string, identity.Prober) {
	t.Helper()
	stateRoot := t.TempDir()
	rec, exact := resultTestRecord(t, stateRoot)
	rec.Authority = line
	data, err := json.Marshal(rec)
	testutil.Require(t, "marshal the record with its authority "+line, err, nil)
	testutil.Require(t, "write the record with its authority "+line,
		os.WriteFile(recordPath(stateRoot), append(data, '\n'), 0o644), nil)
	return stateRoot, resultTestProber{state: identity.Alive, exact: exact}
}
