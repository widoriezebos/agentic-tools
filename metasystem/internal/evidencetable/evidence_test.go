package evidencetable

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/covenant"
	"golang.org/x/sys/unix"
)

// canonicalExample is the inception skill's example, byte-for-byte:
// the two format owners (the skill authors, this parser reads) are
// pinned to each other through it.
const canonicalExample = `# Covenant evidence — toy-app

| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | The app greets the caller by name | greets | repo | bash gate.sh | gate.sh,src/app.py | gate.sh runs the entrypoint and inspects output | observed |
| 2 | Costs stay under the monthly cap | cost-cap | external | tools/cost-report.sh | tools/cost-report.sh | provider billing dashboard, checked at sittings | referenced-not-run |
| 3 | Contradictions reconcile within one cycle | reconcile | repo | (planned) | | no executable proof yet; goal reconcile-proof | planned-floating |

Wired: 2. Floating: 1.
`

func TestParseCanonicalExample(t *testing.T) {
	table, err := Parse([]byte(canonicalExample), "canon")
	if err != nil {
		t.Fatalf("the canonical example must parse: %v", err)
	}
	if len(table.Rows) != 3 {
		t.Fatalf("3 rows expected, got %d", len(table.Rows))
	}
	if table.RecordedWired != 2 || table.RecordedFloating != 1 {
		t.Fatalf("count line misread: wired %d floating %d", table.RecordedWired, table.RecordedFloating)
	}
	first := table.Rows[0]
	if first.CriterionID != "1" || first.ProofID != "greets" || first.Kind != KindRepo {
		t.Fatalf("first row misparsed: %+v", first)
	}
	if len(first.Deps) != 2 || first.Deps[0] != "gate.sh" || first.Deps[1] != "src/app.py" {
		t.Fatalf("deps misparsed: %v", first.Deps)
	}
	if table.Rows[1].Kind != KindExternal || len(table.Rows[1].Deps) != 1 {
		t.Fatalf("external row may declare deps: %+v", table.Rows[1])
	}
	if table.Rows[2].Status != StatusPlannedFloating || len(table.Rows[2].Deps) != 0 {
		t.Fatalf("floating row misparsed: %+v", table.Rows[2])
	}
}

func TestParseEscapedPipe(t *testing.T) {
	doc := strings.Replace(canonicalExample,
		"The app greets the caller by name",
		`greets \| by name`, 1)
	table, err := Parse([]byte(doc), "escaped")
	if err != nil {
		t.Fatalf("escaped pipe must parse: %v", err)
	}
	if table.Rows[0].Criterion != "greets | by name" {
		t.Fatalf("escape not unescaped: %q", table.Rows[0].Criterion)
	}
}

// Every format refusal, named. The table mutates the canonical
// example one law at a time so a passing case proves the law and
// nothing else.
func TestParseFormatRefusals(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"no table", "prose only\n\nWired: 0. Floating: 0.\n", "no evidence table"},
		{"no count line", strings.Replace(canonicalExample, "Wired: 2. Floating: 1.\n", "", 1), "no count line"},
		{"competing count lines", canonicalExample + "\nWired: 2. Floating: 1.\n", "competing count lines"},
		{"competing tables", canonicalExample + "\n| criterion id | criterion | proof id | kind | exact command | repo deps | evidence source | status |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n| 9 | x | y | repo | z | z.sh | s | observed |\n", "competing evidence tables"},
		{"bad status", strings.Replace(canonicalExample, "| observed |", "| verified |", 1), `status "verified"`},
		{"bad kind", strings.Replace(canonicalExample, "| repo | bash gate.sh", "| local | bash gate.sh", 1), `kind "local"`},
		{"bad criterion id", strings.Replace(canonicalExample, "| 1 | The app", "| c 1 | The app", 1), "identity grammar"},
		{"duplicate criterion id", strings.Replace(canonicalExample, "| 2 | Costs", "| 1 | Costs", 1), "already has a row"},
		{"repo without deps", strings.Replace(canonicalExample, "| gate.sh,src/app.py |", "| |", 1), "must declare its repo deps"},
		{"external without source", strings.Replace(canonicalExample, "| provider billing dashboard, checked at sittings |", "| |", 1), "must name its evidence source"},
		{"floating with deps", strings.Replace(canonicalExample, "| (planned) | |", "| (planned) | left.sh |", 1), "declares no deps"},
		{"floating wrong command", strings.Replace(canonicalExample, "| (planned) |", "| make check |", 1), `exactly "(planned)"`},
		{"missing separator", strings.Replace(canonicalExample, "| --- | --- | --- | --- | --- | --- | --- | --- |\n", "", 1), "followed immediately by"},
		{"short separator", strings.Replace(canonicalExample, "| --- | --- | --- | --- | --- | --- | --- | --- |", "| --- | --- |", 1), "8-cell separator"},
		{"second separator", strings.Replace(canonicalExample, "| 2 | Costs", "| --- | --- | --- | --- | --- | --- | --- | --- |\n| 2 | Costs", 1), "second separator"},
		{"misplaced separator behind the count line", strings.Replace(canonicalExample, "| --- | --- | --- | --- | --- | --- | --- | --- |\n", "Wired: 2. Floating: 1.\n| --- | --- | --- | --- | --- | --- | --- | --- |\n", 1), "not the count line"},
		{"blank line where the separator belongs", canonicalExample[:strings.Index(canonicalExample, "| --- |")], "followed immediately by its separator"},
		{"header as the file's last byte", strings.TrimRight(canonicalExample[:strings.Index(canonicalExample, "| --- |")], "\n"), "followed immediately by its separator"},
		{"count overflow", strings.Replace(canonicalExample, "Wired: 2.", "Wired: 99999999999999999999.", 1), "does not fit a number"},
		{"wired planned laundering", strings.Replace(canonicalExample, "| repo | bash gate.sh | gate.sh,src/app.py |", "| repo | (planned) | gate.sh,src/app.py |", 1), "belongs to planned-floating alone"},
		{"unclean dep dotdot", strings.Replace(canonicalExample, "gate.sh,src/app.py", "../escape.sh", 1), `".." components are refused`},
		{"unclean dep absolute", strings.Replace(canonicalExample, "gate.sh,src/app.py", "/etc/passwd", 1), "absolute paths are refused"},
		{"empty dep entry", strings.Replace(canonicalExample, "gate.sh,src/app.py", "gate.sh,,src/app.py", 1), "empty entry"},
		{"cell count", strings.Replace(canonicalExample, "| observed |", "| observed | extra |", 1), "9 cells"},
		{"nul byte", canonicalExample + "\x00", "NUL byte"},
		{"detached row", strings.Replace(canonicalExample, "| 3 | Contradictions", "\n| 3 | Contradictions", 1), "outside the authoritative table"},
	}
	for _, c := range cases {
		_, err := Parse([]byte(c.doc), c.name)
		if err == nil {
			t.Fatalf("%s: must refuse", c.name)
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Fatalf("%s: refusal must name the law: got %v, want substring %q", c.name, err, c.want)
		}
	}
}

func TestWalkerRefusals(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "sub", "real.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "top.sh"), "#!/bin/sh\n")
	if err := os.Symlink("sub", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("top.sh", filepath.Join(root, "alias.sh")); err != nil {
		t.Fatal(err)
	}
	fd, err := OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)

	if err := CheckDep(fd, "sub/real.sh", true); err != nil {
		t.Fatalf("a real entrypoint must pass: %v", err)
	}
	if err := CheckDep(fd, "sub", false); err != nil {
		t.Fatalf("a directory passes in non-entrypoint position: %v", err)
	}
	if err := CheckDep(fd, "sub", true); err == nil {
		t.Fatal("a directory must refuse as the entrypoint")
	}
	if err := CheckDep(fd, "link/real.sh", true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("a symlinked ANCESTOR must refuse by name: %v", err)
	}
	if err := CheckDep(fd, "alias.sh", true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("a symlinked final component must refuse by name: %v", err)
	}
	if err := CheckDep(fd, "gone.sh", true); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("a missing dep must refuse by name: %v", err)
	}
	if err := CheckDep(fd, "top.sh/impossible", false); err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("a file ancestor must refuse by name: %v", err)
	}
	// A FIFO can neither hang the walk (O_NONBLOCK) nor pass as a dep.
	if err := unix.Mkfifo(filepath.Join(root, "pipe"), 0o644); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- CheckDep(fd, "pipe", false) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "neither a regular file nor a directory") {
			t.Fatalf("a FIFO dep must refuse by name: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the FIFO hung the walker — O_NONBLOCK is not honored")
	}
}

// The skill's fenced example IS the canon: this pin fails if the two
// format owners drift apart.
func TestSkillExampleIsTheCanon(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "inception", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "```markdown\n")
	if start < 0 {
		t.Fatal("the skill no longer carries a fenced markdown example")
	}
	rest := text[start+len("```markdown\n"):]
	end := strings.Index(rest, "```")
	if end < 0 {
		t.Fatal("the skill's fenced example never closes")
	}
	skillTable, err := Parse([]byte(rest[:end]), "skill example")
	if err != nil {
		t.Fatalf("the skill's own example must parse under the canon: %v", err)
	}
	ownTable := parseCanon(t)
	if !reflect.DeepEqual(skillTable, ownTable) {
		t.Fatalf("the skill's example and the parser's pinned canon drifted apart:\nskill: %+v\npin:   %+v", skillTable, ownTable)
	}
}

func TestLoadTableRefusesSymlinkedDocs(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(real, "covenant-evidence.md"), canonicalExample)
	if err := os.Symlink("elsewhere", filepath.Join(root, "docs")); err != nil {
		t.Fatal(err)
	}
	fd, err := OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	if _, err := LoadTable(fd, root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("a symlinked docs/ must refuse the table load: %v", err)
	}
}

// judgeBed builds a tree matching the canonical example's repo rows.
func judgeBed(t *testing.T) (int, func()) {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"src", "tools", "docs"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(root, "gate.sh"), "#!/bin/sh\n")
	writeFile(t, filepath.Join(root, "src", "app.py"), "print()\n")
	writeFile(t, filepath.Join(root, "tools", "cost-report.sh"), "#!/bin/sh\n")
	fd, err := OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	return fd, func() { unix.Close(fd) }
}

func parseCanon(t *testing.T) *Table {
	t.Helper()
	table, err := Parse([]byte(canonicalExample), "canon")
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func TestJudgeTraceable(t *testing.T) {
	fd, done := judgeBed(t)
	defer done()
	cov := &covenant.Covenant{
		Identity: covenant.Identity{Name: "toy-app"},
		Requirements: []covenant.Requirement{
			{ID: "1", Ref: "criterion 1: greets by name", Proof: "greets"},
			{ID: "2", Ref: "criterion 2: costs capped", Proof: "cost-cap"},
		},
	}
	report := Judge(cov, parseCanon(t), fd)
	if report.Outcome != "traceable" || len(report.Refusals) != 0 {
		t.Fatalf("must be traceable: %+v", report.Refusals)
	}
	if report.Pairs[0].Verdict != VerdictBound || report.Pairs[0].Assessment != AssessmentRecordedUnverified {
		t.Fatalf("repo pair must be bound and recorded-unverified: %+v", report.Pairs[0])
	}
	if report.Pairs[1].Verdict != VerdictUnjudgedExternal || report.Pairs[1].Assessment != AssessmentRecordedUnverified {
		t.Fatalf("external pair must be unjudged-external and recorded-unverified: %+v", report.Pairs[1])
	}
	// Criterion 3 is a deferred gap: table-only, floating, reported.
	if len(report.Orphans) != 1 || report.Orphans[0].CriterionID != "3" || len(report.Orphans[0].Notes) == 0 {
		t.Fatalf("the deferred criterion must be an orphan with a floating note: %+v", report.Orphans)
	}
	if !report.Counts.Match || report.Counts.DerivedWired != 2 || report.Counts.DerivedFloating != 1 {
		t.Fatalf("counts must match the rows: %+v", report.Counts)
	}
}

func TestJudgeRefusals(t *testing.T) {
	fd, done := judgeBed(t)
	defer done()
	table := parseCanon(t)

	// A requirement with no row refuses with verdict missing-row.
	cov := &covenant.Covenant{
		Identity:     covenant.Identity{Name: "toy-app"},
		Requirements: []covenant.Requirement{{ID: "9", Ref: "criterion 9", Proof: "ghost"}},
	}
	report := Judge(cov, table, fd)
	if report.Outcome != "refused" || report.Pairs[0].Verdict != VerdictMissingRow {
		t.Fatalf("a rowless requirement must refuse missing-row: %+v", report)
	}

	// The wrong proof gets the named near-match diagnosis.
	cov.Requirements = []covenant.Requirement{{ID: "1", Ref: "criterion 1", Proof: "salutes"}}
	report = Judge(cov, table, fd)
	if report.Outcome != "refused" ||
		!strings.Contains(report.Refusals[0].Detail, "bound to proof salutes in the covenant but records proof greets") {
		t.Fatalf("the wrong-proof diagnosis must name both proofs: %+v", report.Refusals)
	}
	if len(report.Orphans) != 2 {
		t.Fatalf("a wrong-proof criterion is backed, not an orphan: %+v", report.Orphans)
	}

	// A covenant-backed floating row refuses regardless of kind.
	cov.Requirements = []covenant.Requirement{{ID: "3", Ref: "criterion 3", Proof: "reconcile"}}
	report = Judge(cov, table, fd)
	if report.Outcome != "refused" || report.Pairs[0].Verdict != VerdictFloating {
		t.Fatalf("covenant-backed floating must refuse: %+v", report.Pairs)
	}
}

func TestJudgeBrokenDep(t *testing.T) {
	fd, done := judgeBed(t)
	defer done()
	doc := strings.Replace(canonicalExample, "gate.sh,src/app.py", "gate.sh,src/gone.py", 1)
	table, err := Parse([]byte(doc), "broken")
	if err != nil {
		t.Fatal(err)
	}
	cov := &covenant.Covenant{
		Identity:     covenant.Identity{Name: "toy-app"},
		Requirements: []covenant.Requirement{{ID: "1", Ref: "criterion 1", Proof: "greets"}},
	}
	report := Judge(cov, table, fd)
	if report.Outcome != "refused" || report.Pairs[0].Verdict != VerdictBrokenDep {
		t.Fatalf("a covenant-backed broken dep must refuse broken-dep: %+v", report.Pairs)
	}

	// An EXTERNAL covenant-backed row's broken declared dep refuses
	// with the same verdict: broken-dep outranks unjudged-external.
	doc = strings.Replace(canonicalExample, "| tools/cost-report.sh | provider", "| tools/gone.sh | provider", 1)
	table, err = Parse([]byte(doc), "broken-external")
	if err != nil {
		t.Fatal(err)
	}
	cov.Requirements = []covenant.Requirement{{ID: "2", Ref: "criterion 2", Proof: "cost-cap"}}
	report = Judge(cov, table, fd)
	if report.Outcome != "refused" || report.Pairs[0].Verdict != VerdictBrokenDep {
		t.Fatalf("a broken external dep must outrank unjudged-external: %+v", report.Pairs)
	}
}

func TestJudgeExternalEntrypointRule(t *testing.T) {
	// The first declared dep is a FILE for every kind: an external
	// row pointing its adapter at a directory refuses.
	fd, done := judgeBed(t)
	defer done()
	doc := strings.Replace(canonicalExample, "| tools/cost-report.sh | provider", "| tools | provider", 1)
	table, err := Parse([]byte(doc), "external-dir")
	if err != nil {
		t.Fatal(err)
	}
	cov := &covenant.Covenant{
		Identity:     covenant.Identity{Name: "toy-app"},
		Requirements: []covenant.Requirement{{ID: "2", Ref: "criterion 2", Proof: "cost-cap"}},
	}
	report := Judge(cov, table, fd)
	if report.Outcome != "refused" || report.Pairs[0].Verdict != VerdictBrokenDep {
		t.Fatalf("an external first dep that is a directory must refuse: %+v", report.Pairs)
	}
}

func TestJudgeOrphanBreakageReports(t *testing.T) {
	fd, done := judgeBed(t)
	defer done()
	doc := strings.Replace(canonicalExample, "gate.sh,src/app.py", "gate.sh,src/gone.py", 1)
	table, err := Parse([]byte(doc), "broken-orphan")
	if err != nil {
		t.Fatal(err)
	}
	cov := &covenant.Covenant{
		Identity:     covenant.Identity{Name: "toy-app"},
		Requirements: []covenant.Requirement{{ID: "2", Ref: "criterion 2", Proof: "cost-cap"}},
	}
	report := Judge(cov, table, fd)
	if report.Outcome != "traceable" {
		t.Fatalf("orphan breakage must not refuse: %+v", report.Refusals)
	}
	var noted bool
	for _, orphan := range report.Orphans {
		if orphan.CriterionID == "1" {
			for _, note := range orphan.Notes {
				noted = noted || strings.Contains(note, "src/gone.py")
			}
		}
	}
	if !noted {
		t.Fatalf("the orphan's broken dep must be noted: %+v", report.Orphans)
	}
}

func TestJudgeCountDrift(t *testing.T) {
	fd, done := judgeBed(t)
	defer done()
	doc := strings.Replace(canonicalExample, "Wired: 2. Floating: 1.", "Wired: 3. Floating: 0.", 1)
	table, err := Parse([]byte(doc), "drift")
	if err != nil {
		t.Fatal(err)
	}
	cov := &covenant.Covenant{
		Identity:     covenant.Identity{Name: "toy-app"},
		Requirements: []covenant.Requirement{{ID: "1", Ref: "criterion 1", Proof: "greets"}},
	}
	report := Judge(cov, table, fd)
	if report.Outcome != "traceable" {
		t.Fatalf("count drift must not refuse: %+v", report.Refusals)
	}
	if report.Counts.Match || len(report.Notes) == 0 {
		t.Fatalf("count drift must be noted: %+v", report.Counts)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
