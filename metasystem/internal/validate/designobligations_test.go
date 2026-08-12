package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const obligationHeader = "| Obligation id | Severity | Design source | Required behavior | Owner | Code proof | Test proof | Runtime proof | Status | Next action |\n" +
	"| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n"

func goodRow(mutations ...func(string) string) string {
	row := "| OBL-1 | HIGH | Requirement | Behavior | `owner.py` | `owner.py` | `test_owner.py` | Not applicable: pure derivation | DONE | None |\n"
	for _, mutate := range mutations {
		row = mutate(row)
	}
	return obligationHeader + row
}

func swap(from, to string) func(string) string {
	return func(row string) string { return strings.Replace(row, from, to, 1) }
}

func TestCheckObligationFile(t *testing.T) {
	cases := []struct {
		name            string
		content         string
		runtimeRequired bool
		want            []string // substrings, one per expected failure, in order
	}{
		{"clean matrix", goodRow(), true, nil},
		{"vague proof", goodRow(swap("`test_owner.py`", "covered somewhere")), false,
			[]string{"OBL-1 cannot be DONE without an owner and concrete code, test, and runtime proof"}},
		{"prose owner and keyword proofs", obligationHeader +
			"| OBL-1 | CRITICAL | Requirement | Behavior | someone will own this | we should test this later | needs testing | manual test pending | DONE | None |\n",
			true, []string{"OBL-1 cannot be DONE"}},
		{"bare not applicable", goodRow(swap("Not applicable: pure derivation", "Not applicable")), false,
			[]string{"OBL-1 cannot be DONE"}},
		{"empty-delimiter not applicable", goodRow(swap("Not applicable: pure derivation", "Not applicable:")), false,
			[]string{"OBL-1 cannot be DONE"}},
		{"unbackticked config path", goodRow(swap("`test_owner.py`", "pyproject.toml")), true, nil},
		{"filename outside old whitelist", goodRow(swap("`test_owner.py`", "module.mjs")), true, nil},
		{"abbreviation prose", goodRow(swap("`test_owner.py`", "compare e.g. the results")), false,
			[]string{"OBL-1 cannot be DONE"}},
		{"fenced matrix is documentation", "```markdown\n" + goodRow() + "```\n", false,
			[]string{"no design-obligation rows found"}},
		{"missing high obligation", goodRow(swap("| DONE |", "| MISSING |")), false,
			[]string{"OBL-1 blocks completion: HIGH MISSING"}},
		{"medium partial passes", goodRow(swap("| HIGH |", "| MEDIUM |"), swap("| DONE |", "| PARTIAL |")), true, nil},
		{"ready without runtime proof", goodRow(swap("| DONE |", "| READY_FOR_RUNTIME |")), false, nil},
		{"ready blocked by runtime-required", goodRow(swap("| DONE |", "| READY_FOR_RUNTIME |")), true,
			[]string{"OBL-1 still needs runtime proof before this gate: READY_FOR_RUNTIME"}},
		{"blocked row", goodRow(swap("| DONE |", "| BLOCKED |")), false,
			[]string{"OBL-1 is BLOCKED and needs the named external decision before this gate"}},
		{"invalid severity", goodRow(swap("| HIGH |", "| SEVERE |")), false,
			[]string{"OBL-1 has invalid severity: SEVERE"}},
		{"invalid status", goodRow(swap("| DONE |", "| SHIPPED |")), false,
			[]string{"OBL-1 has invalid status: SHIPPED"}},
		{"no rows", "no table here\njust prose\n", false, []string{"no design-obligation rows found"}},
		{"empty file", "", false, []string{"no design-obligation rows found"}},
		{"prose interrupts then table resumes", goodRow() + "prose interrupts\n" + goodRow(), false, nil},
		{"camelcase owner", goodRow(swap("| `owner.py` | `owner.py` |", "| OwnerThing | `owner.py` |")), false, nil},
		{"double-colon owner", goodRow(swap("| `owner.py` | `owner.py` |", "| pkg::Owner | `owner.py` |")), false, nil},
		{"weak owner", goodRow(swap("| `owner.py` | `owner.py` |", "| TBD | `owner.py` |")), false,
			[]string{"OBL-1 cannot be DONE"}},
	}
	for _, tc := range cases {
		failures := checkObligationFile("plan.md", tc.content, tc.runtimeRequired)
		if len(failures) != len(tc.want) {
			t.Fatalf("%s: got %d failures %v, want %d", tc.name, len(failures), failures, len(tc.want))
		}
		for i, want := range tc.want {
			if !strings.Contains(failures[i], want) {
				t.Fatalf("%s: failure %d = %q, want substring %q", tc.name, i, failures[i], want)
			}
			if !strings.HasPrefix(failures[i], "plan.md:") {
				t.Fatalf("%s: failure %d lacks the file:line prefix: %q", tc.name, i, failures[i])
			}
		}
	}
}

func TestDesignObligations(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "plans"), 0o755)
	os.WriteFile(filepath.Join(root, "plans", "matrix.md"), []byte(goodRow()), 0o644)
	bad := filepath.Join(root, "bad.md")
	os.WriteFile(bad, []byte(goodRow(swap("| DONE |", "| MISSING |"))), 0o644)

	// A relative path unreadable from the working directory resolves
	// against root; the messages keep the caller's original argument.
	out, errs, code := DesignObligations(root, []string{"plans/matrix.md"}, true)
	if code != 0 || len(errs) != 0 || out[0] != "design obligation gate passed (1 file(s), runtime_required=1)" {
		t.Fatalf("root-relative pass wrong: %v %v %d", out, errs, code)
	}
	out, errs, code = DesignObligations(root, []string{bad, "absent.md"}, false)
	if code != 1 || len(out) != 0 {
		t.Fatalf("failing gate wrong: %v %v %d", out, errs, code)
	}
	if !strings.Contains(errs[0], "OBL-1 blocks completion") ||
		errs[1] != "missing or unreadable obligation file: absent.md" ||
		errs[2] != "design obligation gate failed" {
		t.Fatalf("failure lines wrong: %v", errs)
	}
}
