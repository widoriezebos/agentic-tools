package batch

import (
	"strings"
	"testing"
)

func TestTrunkRedIdentity(t *testing.T) {
	failed := func(classname, name, reason string) Failure {
		return Failure{Report: "report", Classname: classname, Name: name, Status: "failed", Reason: reason}
	}
	skipped := Failure{Report: "report", Classname: "Class", Name: "skip", Status: "skipped"}
	passed := Failure{Report: "report", Classname: "Class", Name: "pass", Status: "passed"}
	leftRed := RedGroup{ID: "group", Failures: []Failure{failed("Class", "two", "reason 1"), failed("Class", "one", "reason 2")}}
	rightRed := RedGroup{ID: "group", Failures: []Failure{failed("Class", "one", "changed"), failed("Class", "two", "changed")}}
	tests := []struct {
		name        string
		left, right RedGroup
		same        bool
		exact       string
	}{
		{name: "same failing names on another tree and attempt", left: leftRed, right: rightRed, same: true},
		{name: "two groups", left: RedGroup{ID: "one", Failures: []Failure{failed("Class", "test", "")}}, right: RedGroup{ID: "two", Failures: []Failure{failed("Class", "test", "")}}, same: false},
		{name: "same name different classname", left: RedGroup{ID: "group", Failures: []Failure{failed("One", "test", "")}}, right: RedGroup{ID: "group", Failures: []Failure{failed("Two", "test", "")}}, same: false},
		{name: "changed failure reason", left: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", "one")}}, right: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", "two")}}, same: true},
		{name: "no failing test ignores different reasons", left: RedGroup{ID: "group", Status: "runaway", NotRunReason: "supervisor stalled"}, right: RedGroup{ID: "group", Status: "runaway", NotRunReason: "supervisor stalled; /private/tmp/u0seam.1234/dump.txt"}, same: true},
		{name: "no failing test keeps status", left: RedGroup{ID: "group", Status: "runaway"}, right: RedGroup{ID: "group", Status: "invalid"}, same: false},
		{name: "same classname and report different name", left: RedGroup{ID: "group", Failures: []Failure{failed("Class", "one", "")}}, right: RedGroup{ID: "group", Failures: []Failure{failed("Class", "two", "")}}, same: false},
		{name: "same classname and name different report", left: RedGroup{ID: "group", Failures: []Failure{{Report: "one", Classname: "Class", Name: "test", Status: "failed"}}}, right: RedGroup{ID: "group", Failures: []Failure{{Report: "two", Classname: "Class", Name: "test", Status: "failed"}}}, same: false},
		{name: "passed excluded from failed names", left: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", "")}}, right: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", ""), passed}}, same: true},
		{name: "duplicate failing line removed", left: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", "")}}, right: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", ""), failed("Class", "test", "")}}, same: true},
		{name: "skipped excluded from failed names", left: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", "")}}, right: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", ""), skipped}}, same: true},
		{name: "only skipped uses status", left: RedGroup{ID: "group", Status: "blocked", NotRunReason: "one"}, right: RedGroup{ID: "group", Status: "blocked", NotRunReason: "two", Failures: []Failure{skipped}}, same: true},
		{name: "tab placement cannot collide", left: RedGroup{ID: "group", Failures: []Failure{failed("Class\tpart", "test", "")}}, right: RedGroup{ID: "group", Failures: []Failure{failed("Class", "part\ttest", "")}}, same: false},
		{name: "normalized group names cannot collide", left: RedGroup{ID: "a/b", Failures: []Failure{failed("Class", "test", "")}}, right: RedGroup{ID: "a-b", Failures: []Failure{failed("Class", "test", "")}}, same: false},
		{name: "pinned exact id", left: RedGroup{ID: "group", Failures: []Failure{failed("Class", "test", "")}}, exact: "tr-group-fb7c85dbe5a5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			left := TrunkRedID(test.left)
			if test.exact != "" {
				if left != test.exact {
					t.Fatalf("id %q, want %q", left, test.exact)
				}
				return
			}
			right := TrunkRedID(test.right)
			if (left == right) != test.same {
				t.Fatalf("ids %q and %q same=%v, want %v", left, right, left == right, test.same)
			}
		})
	}
	if got := TrunkRedID(RedGroup{ID: "suite/one", Status: "invalid"}); strings.Contains(got, "/") {
		t.Fatalf("slash remains in id %q", got)
	}
}
