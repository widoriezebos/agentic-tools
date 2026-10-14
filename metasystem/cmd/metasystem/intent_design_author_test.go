package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

func TestDesignReviewAutomaticFinalRevision(t *testing.T) {
	t.Parallel()
	b, dir, returned := designEvidenceBed(t, evidenceInventory, acceptanceUnits)
	returned["findings"], returned["rigor"], returned["verdictMaterialCount"] = []any{evidenceFinding("F1", "## Collection:1", "")}, []any{evidenceRigor("F1")}, 1
	b.writeJSON(filepath.Join(dir, "return.json"), returned)
	b.writeFile(filepath.Join(dir, "return.md"), "VERDICT: REVISE material=1\n")
	template := b.decide(b.review(), map[string]string{"F1": "accepted | concrete requirement | ## Collection:1"})
	prior := string(mustRead(t, b.design))
	proposal := suppliedFoldPage(t, prior, concreteFoldMapping())
	proposal += "Scope item: {\"Unit\":\"reader\",\"Decisions\":[\"## Collection:1\"],\"DependsOn\":[]}\n"
	work := newWorkBed(t)
	work.manager.Prober = &treeProber{dead: true}
	owners := b.intentBed.terminalOwners()
	owners.delivery = b.owners
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner { return &launch.UnitRunner{Manager: work.manager} }
	work.starter.author = func(record launch.Record) {
		command, err := launch.ClaudeHeadless{}.Command(record, t.TempDir())
		if err != nil {
			t.Error(err)
			return
		}
		output, _, err := authorPromptTask(command.Stdin)
		if err != nil {
			t.Error(err)
			return
		}
		if !strings.Contains(command.Stdin, "Exit operation:") || strings.Contains(command.Stdin, "First version.") {
			t.Error("the author request lacks its operation or duplicates the frozen page")
		}
		b.writeFile(output, proposal)
	}
	args := []string{"design", "review", b.design, "--tool-calls", "30", "--dispositions", template}
	if code, result := b.runJSON(owners, args...); code != 0 {
		encoded, _ := json.Marshal(result)
		t.Fatalf("automatic revision: %d %s", code, encoded)
	}
	exits := b.goalFile(bedGoal).DesignExits
	attempts, err := work.manager.DesignAttempts(b.design)
	if err != nil || len(exits) != 1 || len(attempts) != 1 || exits[0].AuthorAttempt != attempts[0].Attempt || exits[0].Operation != attempts[0].Operation || attempts[0].Outcome != "published" || exits[0].Expected != prior || b.closes != 1 {
		t.Fatalf("author and committed exit disagree: exits=%+v attempts=%+v closes=%d err=%v", exits, attempts, b.closes, err)
	}
	acceptanceBuild(t, b, true)
	if code, result := b.runJSON(owners, args...); code != 0 || result.Outcome != intentUnchanged || len(work.starter.launched()) != 1 || b.closes != 1 {
		t.Fatalf("replay repeated the author or closure: %d %+v", code, result)
	}
}
