package goal

import (
	"strings"
	"testing"
)

// The grammar-tightening fold (review r1, F14): the closed file and
// root grammars refuse what the design names — oversized ids, wrong
// timestamp forms, duplicate fields, records on the wrong state, and
// root fields that are shaped rather than merely nonempty.

func TestGoalIdCapsAtOneHundredCharacters(t *testing.T) {
	if !validId(strings.Repeat("a", 100)) {
		t.Fatal("one hundred characters is the lawful maximum")
	}
	if validId(strings.Repeat("a", 101)) {
		t.Fatal("the filesystem-safety cap is 100 characters")
	}
}

func TestFileGrammarRefusesTheMalformed(t *testing.T) {
	for _, leg := range []struct {
		name     string
		mutate   func(*GoalFile)
		fragment string
	}{
		{"a Parked record on a queued goal", func(f *GoalFile) {
			f.Parked = &ParkRecord{By: "human:wido", At: "2026-08-20T10:05:00Z", Because: "smuggled"}
		}, "Parked record on a queued goal"},
		{"a missing Intent", func(f *GoalFile) { f.Intent = "" }, "missing Intent"},
		{"a missing OpenedAt", func(f *GoalFile) { f.OpenedAt = "" }, "missing OpenedAt"},
		{"a non-RFC3339 OpenedAt", func(f *GoalFile) { f.OpenedAt = "yesterday" }, "not an RFC3339 timestamp"},
		{"a non-RFC3339 claim stamp", func(f *GoalFile) {
			f.State = StateClaimed
			f.Claimed = &ClaimRecord{Machine: "mac-a", Lineage: "lin-1", At: "soon"}
		}, "Claimed at=\"soon\" is not an RFC3339 timestamp"},
	} {
		t.Run(leg.name, func(t *testing.T) {
			f := vGoal("grammar-probe", StateQueued)
			leg.mutate(f)
			_, problems := ParseFile(RenderFile(f))
			if !problemsContain(problems, leg.fragment) {
				t.Fatalf("refusal names the rule: %v", problems)
			}
		})
	}
}

func TestDuplicateFieldsRefuseByName(t *testing.T) {
	// A second State line must refuse, not silently win.
	rendered := string(RenderFile(vGoal("dup-probe", StateQueued)))
	doctored := strings.Replace(rendered, "- State: queued", "- State: queued\n- State: done", 1)
	doctored = withFreshIntegrity(doctored)
	_, problems := ParseFile([]byte(doctored))
	if !problemsContain(problems, "duplicate field \"State\"") {
		t.Fatalf("a duplicate field refuses by name: %v", problems)
	}

	rootRendered := string(RenderRoot(rootGolden()))
	rootDoctored := strings.Replace(rootRendered, "- FormatVersion: 1", "- FormatVersion: 1\n- FormatVersion: 1", 1)
	rootDoctored = withFreshIntegrity(rootDoctored)
	_, rootProblems := ParseRoot([]byte(rootDoctored))
	if !problemsContain(rootProblems, "duplicate field \"FormatVersion\"") {
		t.Fatalf("a duplicate root field refuses by name: %v", rootProblems)
	}
}

func TestRecordGrammarIsClosed(t *testing.T) {
	// An unknown key inside a Claimed record refuses; a stray token
	// refuses; History timestamps bind to RFC3339.
	f := vGoal("closed-probe", StateClaimed)
	rendered := string(RenderFile(f))
	doctored := withFreshIntegrity(strings.Replace(rendered, "at=2026-08-20T10:05:00Z", "at=2026-08-20T10:05:00Z smuggled=yes", 1))
	_, problems := ParseFile([]byte(doctored))
	if !problemsContain(problems, "unknown key \"smuggled\"") {
		t.Fatalf("the record grammar is closed: %v", problems)
	}

	if _, err := ParseHistoryLine("- lately 01J5X0000000000000000000A0-mac-a-1a2b3c4d open actor=mac-a+lin-1"); err == nil || !strings.Contains(err.Error(), "not RFC3339") {
		t.Fatalf("a History timestamp binds to RFC3339: %v", err)
	}
}

func TestRootFieldsAreShapedNotMerelyNonempty(t *testing.T) {
	for _, leg := range []struct {
		name     string
		mutate   func(*RootRecord)
		fragment string
	}{
		{"a malformed Identity", func(r *RootRecord) { r.Identity = "not-a-ulid" }, "not ULID-shaped"},
		{"an unknown FormatVersion", func(r *RootRecord) { r.FormatVersion = "2" }, "FormatVersion \"2\" is not 1"},
		{"a malformed MigrationEpoch", func(r *RootRecord) { r.MigrationEpoch = "August" }, "not an RFC3339 timestamp"},
		{"a malformed ManifestDigest", func(r *RootRecord) { r.ManifestDigest = "abc123" }, "not a sha256 hex digest"},
		{"an unknown MigrationMode", func(r *RootRecord) { r.MigrationMode = "vibes" }, "not manifest|bare"},
	} {
		t.Run(leg.name, func(t *testing.T) {
			r := rootGolden()
			leg.mutate(r)
			_, problems := ParseRoot(RenderRoot(r))
			if !problemsContain(problems, leg.fragment) {
				t.Fatalf("refusal names the shape: %v", problems)
			}
		})
	}
}

// withFreshIntegrity re-stamps a doctored rendering so the byte
// tamper under test is the GRAMMAR problem, not the digest.
func withFreshIntegrity(doc string) string {
	lines := strings.Split(strings.TrimRight(doc, "\n"), "\n")
	body := strings.Join(lines[:len(lines)-1], "\n") + "\n"
	return body + "Integrity: sha256=" + sha256HexBytes([]byte(body)) + "\n"
}
