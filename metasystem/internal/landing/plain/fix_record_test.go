package plain

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestLaneFixRealRecordAndBriefShape(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/lane-fix-20261011-record.json")
	if err != nil {
		t.Fatal(err)
	}
	var merge Fix
	if err := json.Unmarshal(raw, &merge); err != nil {
		t.Fatal(err)
	}
	if merge.Attempt != "20261011T022722.620392000Z" || merge.Round != 0 || len(merge.Paths) != 3 || len(merge.Generated) != 1 || merge.CheckResult != "green" || merge.Unit() != "" {
		t.Fatalf("real merge record changed shape: %+v", merge)
	}
	root := t.TempDir()
	if err := WriteFix(root, &merge); err != nil {
		t.Fatal(err)
	}
	retained, err := FixForAttempt(root, merge.Attempt)
	if err != nil || retained == nil || retained.Brief != merge.Brief || retained.Generated[0].Command[0] != "npm" {
		t.Fatalf("round trip %+v: %v", retained, err)
	}
	brief, err := os.ReadFile("testdata/lane-fix-20261011-brief.md")
	if err != nil {
		t.Fatal(err)
	}
	attempt := "20261011T040737.033326000Z"
	if !strings.Contains(string(brief), "Proof attempt `"+attempt+"`") || !strings.Contains(string(brief), "never loosen or delete a test") {
		t.Fatal("real red brief lost its proof or repair constraint")
	}
	repair := Fix{Attempt: attempt, Round: 1, Parent: merge.Commit, Goal: merge.Goal, Units: []string{"metasystem/cmd/metasystem"}}
	if repair.Unit() != "lane-fix-20261011T040737-1" {
		t.Fatalf("repair identity %q", repair.Unit())
	}
	for _, attempt := range []string{"20261011T04073.033326000Z", "20261311T040737.033326000Z", "20261011T246737.033326000Z"} {
		repair.Attempt = attempt
		if repair.Unit() != "" {
			t.Fatalf("malformed proof stamp admitted: %s", attempt)
		}
	}
}
