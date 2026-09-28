package audit

import (
	"slices"
	"strings"
	"testing"
)

// The 2026-09-28 U9 hook rename (030a185c0, declared by hand in 5de1a3eab):
// only the command text inside strings.Count changed; the decision literal,
// the comparison and every other token stayed.
const (
	u9RenameBefore = "if strings.Count(text, \"supervision-hook.sh claude stop\") != 1 || strings.Contains(text, `\\\"decision\\\":\\\"block\\\"`) || !strings.Contains(text, jsonString(t, want)) {\n\tt.Fatal(text)\n}\n"
	u9RenameAfter  = "if strings.Count(text, \"internal hook claude stop;\") != 1 || strings.Contains(text, `\\\"decision\\\":\\\"block\\\"`) || !strings.Contains(text, jsonString(t, want)) {\n\tt.Fatal(text)\n}\n"
)

func rewordFixture(t *testing.T, before, after string) StopSurfaceResult {
	t.Helper()
	fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "setup_test.go"}}, map[string]string{
		"setup_test.go": goStopSurfaceFixture("fixture", before),
	}, true)
	fixture.write("setup_test.go", goStopSurfaceFixture("fixture", after))
	return fixture.audit(stopSurfaceTestOptions())
}

// TestStopSurfaceAdmitsTheU9RenameAsAReword: a changed assertion line whose
// Stop-decision content is unchanged is reported as reworded and needs no
// declaration or goal permission.
func TestStopSurfaceAdmitsTheU9RenameAsAReword(t *testing.T) {
	t.Parallel()
	result := rewordFixture(t, u9RenameBefore, u9RenameAfter)
	if result.Refused() || len(result.Removed) != 0 || len(result.Added) != 0 || len(result.Moved) != 0 {
		t.Fatalf("the U9 rename was refused or counted as a move: %+v", result)
	}
	if len(result.Reworded) != 1 || result.Reworded[0].File != "setup_test.go" ||
		!strings.Contains(result.Reworded[0].From, "supervision-hook.sh claude stop") ||
		!strings.Contains(result.Reworded[0].To, "internal hook claude stop;") {
		t.Fatalf("reworded = %+v", result.Reworded)
	}
	if got := result.Summary(); !strings.HasSuffix(got, "added 0, moved 0, removed 0; reworded 1") {
		t.Fatalf("summary = %q", got)
	}
}

// TestStopSurfaceRewordNeverCoversADecisionChange: flipping the comparison,
// turning block into allow, deleting the assertion, touching a Stop
// identifier or its operand, or rewording text into decision vocabulary all
// stay moves that need a permitted goal's declaration.
func TestStopSurfaceRewordNeverCoversADecisionChange(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name, before, after string
		removed, added      int
	}{
		{"comparison flipped", u9RenameBefore, strings.Replace(u9RenameAfter, "!= 1", "== 1", 1), 1, 1},
		{"renamed and flipped", u9RenameBefore, strings.Replace(u9RenameBefore, ") != 1", ") == 1", 1), 1, 1},
		{"block becomes allow", u9RenameBefore, strings.Replace(u9RenameAfter, `\"block\"`, `\"allow\"`, 1), 1, 1},
		{"assertion deleted", u9RenameBefore, "t.Log(text)\n", 1, 0},
		{"Stop identifier operand changed", "if !got.ShouldBlock || got.Reason != \"idle\" { t.Fatal() }\n", "if !other.ShouldBlock || got.Reason != \"idle\" { t.Fatal() }\n", 1, 1},
		{"negation dropped", "if !got.ShouldBlock || got.Reason != \"idle\" { t.Fatal() }\n", "if got.ShouldBlock || got.Reason != \"idle\" { t.Fatal() }\n", 1, 1},
		{"text reworded into decision vocabulary", "if !got.ShouldBlock || got.Reason != \"idle\" { t.Fatal() }\n", "if !got.ShouldBlock || got.Reason != \"blocked idle\" { t.Fatal() }\n", 1, 1},
		{"decision vocabulary reworded out", "if !got.ShouldBlock || got.Reason != \"refusal\" { t.Fatal() }\n", "if !got.ShouldBlock || got.Reason != \"idle\" { t.Fatal() }\n", 1, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			result := rewordFixture(t, c.before, c.after)
			if !result.Refused() || len(result.Reworded) != 0 || len(result.Removed) != c.removed || len(result.Added) != c.added {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

// TestStopSurfaceRewordPairsOneForOne: a reword pairs one removed line with
// one added line of the same file; a second removal of the same assertion is
// still a move.
func TestStopSurfaceRewordPairsOneForOne(t *testing.T) {
	t.Parallel()
	result := rewordFixture(t, u9RenameBefore+u9RenameBefore, u9RenameAfter)
	if !result.Refused() || len(result.Reworded) != 1 || len(result.Removed) != 1 || len(result.Added) != 0 {
		t.Fatalf("result = %+v", result)
	}
	want := StopSurfaceLine{File: "setup_test.go", Line: strings.Join(strings.Fields(strings.SplitN(u9RenameBefore, "\n", 2)[0]), " ")}
	if !slices.Contains(result.Removed, want) {
		t.Fatalf("removed = %+v, want %+v", result.Removed, want)
	}
}

// TestStopSurfaceRewordStaysOutOfFixtureBeds: shell fixture beds are not
// tokenized, so every changed bed line stays a move.
func TestStopSurfaceRewordStaysOutOfFixtureBeds(t *testing.T) {
	t.Parallel()
	const path = "scripts/agents/bed-fixtures.sh"
	fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "bed", Path: path}}, map[string]string{
		path: "#!/usr/bin/env bash\nexpect \"decision\":\"block\" \"old words\"\n",
	}, true)
	fixture.write(path, "#!/usr/bin/env bash\nexpect \"decision\":\"block\" \"new words\"\n")
	result := fixture.audit(stopSurfaceTestOptions())
	if !result.Refused() || len(result.Reworded) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

// TestStopSurfaceRewordStaysInsideOneFile: an assertion removed from one file
// and its reworded twin added to another is a removal and an addition.
func TestStopSurfaceRewordStaysInsideOneFile(t *testing.T) {
	t.Parallel()
	fixture := newStopSurfaceFixture(t, []stopSurfaceFile{{Kind: "go", Path: "setup_test.go"}}, map[string]string{
		"setup_test.go": goStopSurfaceFixture("fixture", u9RenameBefore),
	}, true)
	fixture.write("setup_test.go", goStopSurfaceFixture("fixture", ""))
	fixture.write("other_test.go", goStopSurfaceFixture("fixture", u9RenameAfter))
	result := fixture.audit(stopSurfaceTestOptions())
	if !result.Refused() || len(result.Reworded) != 0 || len(result.Removed) != 1 || len(result.Added) != 1 {
		t.Fatalf("result = %+v", result)
	}
}
