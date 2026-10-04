package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	refusalreg "github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
)

func TestGateRecordsAreGoverned(t *testing.T) {
	t.Parallel()
	ids := map[string]bool{}
	for _, id := range refusalreg.GovernedBy {
		ids[id] = true
	}
	t.Run("sweep", func(t *testing.T) {
		t.Parallel()
		data, err := os.ReadFile("testdata/gate-records.md")
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		writeRulingRegister(t, root, strings.Split(strings.TrimSpace(string(data)), "\n"))
		if err := sweepRulingReviews(root, time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
		digest, err := os.ReadFile(narratordigest.Path(root))
		if err != nil {
			t.Fatal(err)
		}
		for id := range ids {
			if !strings.Contains(string(digest), id+" owner=Wido") {
				t.Errorf("review due for %s was not flagged: %s", id, digest)
			}
		}
	})
	t.Run("live", func(t *testing.T) {
		t.Parallel()
		register, err := rulings.Read(filepath.Join("..", ".."))
		if err != nil {
			t.Fatal(err)
		}
		for id := range ids {
			var row *rulings.Row
			for i := range register.Rows {
				if register.Rows[i].ID == id {
					row = &register.Rows[i]
					break
				}
			}
			if row == nil {
				t.Logf("%s not in the register yet", id)
				continue
			}
			if row.Owner == "" || row.Class == "" || row.Due == "" {
				t.Errorf("%s needs an owner, typed review class and due date: %+v", id, row)
			}
			for _, label := range []string{"MUST REFUSE", "APPEAL:", "IF THE CHECK BREAKS:"} {
				if !strings.Contains(row.Words, label) {
					t.Errorf("%s lacks %s", id, label)
				}
			}
		}
	})
}
