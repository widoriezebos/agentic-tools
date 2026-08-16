package adapter

import "testing"

// The dialect refusal and devin's evidenced mappings: read-only
// rides ask (probe step C), runtime-default rides accept-edits
// (step D), and bypass is unreachable from any grade.
func TestACPDialects(t *testing.T) {
	if _, err := ACPDialectFor("nonesuch"); err == nil {
		t.Fatal("an undeclared runtime must refuse, never fall back")
	}
	dialect, err := ACPDialectFor("devin")
	if err != nil {
		t.Fatal(err)
	}
	if dialect.ModeForTools["read-only"] != "ask" || dialect.ModeForTools["runtime-default"] != "accept-edits" {
		t.Fatalf("devin's evidenced mapping drifted: %+v", dialect.ModeForTools)
	}
	for _, mode := range dialect.ModeForTools {
		if mode == "bypass" || mode == "smart" {
			t.Fatalf("no envelope grade may reach %s", mode)
		}
	}
	list := ACPDialectList()
	if len(list) == 0 || list[0] != "devin" {
		t.Fatalf("dialect list: %v", list)
	}
}
