package refusal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegisterSurvivesLinesInsertedAboveARefusal is the anchor's witness:
// the register pins each refusal by a stable symbol, so inserting lines
// above every refusal site (an edit that broke the file:line register a
// dozen times on 2026-09-28) leaves every row resolving to its emission.
func TestRegisterSurvivesLinesInsertedAboveARefusal(t *testing.T) {
	t.Parallel()
	root := moduleRoot(t)
	identifiers := refusalCodeIdentifiers(t, root)
	const inserted = "// an unrelated edit above the refusal\n// adds lines\n// and moves every line below it\n"
	for _, row := range Rows {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(row.Owner), filepath.FromSlash(SiteFile(row.Site))))
		if err != nil {
			t.Fatalf("row %s: %v", row.Code, err)
		}
		// The lines go after the package clause, so the file still parses.
		clause := strings.Index(string(data), "\n")
		shifted := append([]byte(string(data[:clause+1])+inserted), data[clause+1:]...)
		if problem := rowSiteProblem(t, row, shifted, identifiers[row.Code]); problem != "" {
			t.Errorf("row %s site %q after lines were inserted above it: %s", row.Code, row.Site, problem)
		}
	}
}

// TestSiteAnchorsNameSymbolsNotLines: a site is file#Symbol; a line-number
// site, an unknown symbol and a symbol that is not declared at the top level
// are refused.
func TestSiteAnchorsNameSymbolsNotLines(t *testing.T) {
	t.Parallel()
	source := []byte("package fixture\n\nconst code = \"FIXTURE_REFUSED\"\n\nvar table = map[string]string{\"k\": \"TABLE_REFUSED\"}\n\ntype engine struct{}\n\nfunc (e *engine) admit() error {\n\treturn fmt.Errorf(\"ADMIT_REFUSED\")\n}\n\nfunc plain() error {\n\tinner := func() error { return errors.New(code) }\n\treturn inner()\n}\n")
	for site, want := range map[string][2]int{
		"fixture.go#engine.admit": {9, 11},
		"fixture.go#plain":        {13, 16},
		"fixture.go#table":        {5, 5},
		"fixture.go#code":         {3, 3},
	} {
		first, last, err := siteSpan(site, source)
		if err != nil || first != want[0] || last != want[1] {
			t.Errorf("siteSpan(%q) = %d, %d, %v; want %v", site, first, last, err, want)
		}
	}
	for _, site := range []string{"fixture.go:10", "fixture.go#missing", "fixture.go#inner", "fixture.go#", "fixture.go#admit"} {
		if _, _, err := siteSpan(site, source); err == nil {
			t.Errorf("siteSpan(%q) resolved; want a refusal", site)
		}
	}
}
