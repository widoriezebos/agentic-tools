package processchange

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
)

func TestDeclarationCheckSettingsAdmission(t *testing.T) {
	t.Parallel()
	for _, index := range []int{4, 5} {
		t.Run(launch.UnitDeclarationKeys()[index], func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			main := Declaration{Commit: "main", Values: [6]string{"cheap", "audit", "15", "full"}}
			candidate := main
			candidate.Values = candidate.EffectiveValues()
			published := false
			admission := DeclarationAdmission{Check: Check{Root: root, Installation: roots.Installation(root), Now: time.Unix(1, 0),
				ProcessAct: ProcessAct{Goal: "goal", Unit: "unit", Operation: "round", Checkout: root}, Remedy: func(string) string { return "approve check" }},
				Source:  func() (Declaration, Declaration, Declaration, error) { return candidate, main, main, nil },
				Recheck: func() error { return nil }, Publish: func(ProcessAct) error { published = true; return nil }, Impact: func() error { return nil }}
			// Retain an actual four-value baseline to exercise migration at admission.
			baseline := `{"Goal":"goal","Seed":"main","Snapshot":{"Commit":"main","Values":["cheap","audit","15","full"]}}`
			path := filepath.Join(root, "process", fmt.Sprintf("declaration-%x.json", sha256.Sum256([]byte("goal"))))
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(baseline), 0600); err != nil {
				t.Fatal(err)
			}
			act, err := AdmitDeclaration(admission)
			if err != nil || act.Status != "unchanged" || !published {
				t.Fatal("explicit defaults changed an old declaration's meaning")
			}
			candidate.Values[index] = map[int]string{4: "bounded", 5: "9"}[index]
			published = false
			act, err = AdmitDeclaration(admission)
			if index == 4 {
				if err == nil || !strings.Contains(err.Error(), "bounded") || published || act.Status == "proposed" {
					t.Fatal("unavailable kind was admitted or proposed")
				}
				candidate.Values[4], candidate.Values[5] = "legacy", "9"
				act, err = AdmitDeclaration(admission)
			}
			if err != nil || act.Status != "proposed" || published || act.AfterDeclaration.Values[index] != candidate.Values[index] {
				t.Fatalf("changed check setting bypassed admission: status=%s error=%v", act.Status, err)
			}
			admission.Person, admission.Act = true, act.ID
			act, err = AdmitDeclaration(admission)
			if err != nil || act.Status != "applied" || !published {
				t.Fatalf("exact approval did not apply: status=%s error=%v", act.Status, err)
			}
			admission.Resume, admission.After, admission.AfterDeclaration = true, candidate.Values[0], &candidate
			if _, err := AdmitDeclaration(admission); err != nil {
				t.Fatal(err)
			}
			candidate.Values[index] = map[int]string{4: "bounded", 5: "10"}[index]
			if _, err := AdmitDeclaration(admission); err == nil {
				t.Fatal("an approved act authorized different frozen check settings")
			}
		})
	}
}

func TestDeclarationCheckSettingsInverse(t *testing.T) {
	t.Parallel()
	for _, row := range []struct{ name, before, inherited, after, want string }{
		{"absent defaults", "", "", "proof.check-kind=bounded\nproof.check-budget=9\n", ""},
		{"explicit defaults", "proof.check-kind=legacy\nproof.check-budget=6\n", "", "proof.check-kind=bounded\nproof.check-budget=9\n", "proof.check-kind=legacy\nproof.check-budget=6\n"},
		{"inherited kind", "", "proof.check-kind=bounded\n", "proof.check-kind=bounded\nproof.check-budget=9\n", "proof.check-kind=bounded\n"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			base := "# preserve this comment\nproof.cheap=cheap\nproof.audits=audit\nproof.deadline=15\nproof.full=full\n"
			declaration := func(extra string) *Declaration {
				d := &Declaration{Branch: "goal/goal", Content: base + extra}
				for i, key := range launch.UnitDeclarationKeys() {
					d.Values[i], _, _ = config.CommittedContentLookup(d.Content, key)
				}
				return d
			}
			source := ProcessAct{ID: "approved", Status: "applied", Goal: "goal", Checkout: root,
				BeforeDeclaration: declaration(row.before), AfterDeclaration: declaration(row.after), InheritedDeclaration: declaration(row.inherited)}
			path := filepath.Join(root, "process", "acts", source.ID+".json")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(source)
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			request := ProcessAct{Goal: source.Goal, Checkout: root}
			if _, err := PrepareInverse(root, roots.Installation(root), source.ID, "goal/goal", "metasystem.conf", base+strings.ReplaceAll(row.after, "=9", "=10"), nil, request, time.Unix(1, 0)); err == nil {
				t.Fatal("inverse overwrote a later budget change")
			}
			act, err := PrepareInverse(root, roots.Installation(root), source.ID, "goal/goal", "metasystem.conf", base+row.after, nil, request, time.Unix(1, 0))
			if err != nil || act.AfterDeclaration == nil || act.AfterDeclaration.Content != base+row.want || len(act.Patch) == 0 {
				t.Fatalf("inverse did not restore only the owned settings: %v", err)
			}
			if act.AfterDeclaration.EffectiveValues() != declaration(row.want).EffectiveValues() {
				t.Fatal("restored declaration disagrees with restored configuration")
			}
		})
	}
}
