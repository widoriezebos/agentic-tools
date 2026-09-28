package adapter

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/helm"
)

// Under the helm the tool gate is silent even where it would deny: no
// output, one decision row with cause helm.
func TestToolGateSilentUnderHelm(t *testing.T) {
	t.Parallel()
	t.Run("HM-6", func(t *testing.T) {
		root, transcript := toolGateFixture(t, 120000)
		if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
			t.Fatal(err)
		}
		birth := time.Date(2026, 9, 28, 19, 20, 0, 0, time.UTC)
		if _, err := helm.Write(root, helm.Record{By: "Wido", At: birth.Format(time.RFC3339), Reason: "by hand"}); err != nil {
			t.Fatal(err)
		}
		stdout := &bytes.Buffer{}
		if err := RunToolGate(toolGateOptions(root, transcript, "deny", birth, func() time.Time { return birth.Add(time.Millisecond) }, stdout)); err != nil || stdout.Len() != 0 {
			t.Fatalf("tool gate under the helm: err=%v stdout=%q", err, stdout.String())
		}
		if rows := readToolGateRows(t, root); len(rows) != 1 || rows[0].Cause != "helm" || rows[0].Decision != "allow" || rows[0].Tokens != 0 {
			t.Fatalf("decision rows = %#v, want one allow row with cause helm and no transcript read", rows)
		}
	})
}
