package launch

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDeclaredAreas(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, text string
		want       []string
		known, bad bool
	}{
		{"union", "- Areas: [\"./metasystem/internal/a/\", \"file with, spaces.go\"]\n\n| Unit | Lines | Areas |\n| --- | --- | --- |\n| U1 | 20 | [\"metasystem\\\\internal\\\\b/**\", \"metasystem/internal/a/\"] |\n", []string{"metasystem/internal/a/", "file with, spaces.go", "metasystem/internal/b/**"}, true, false},
		{"leading class", "Areas: [ab]/*.go", []string{"[ab]/*.go"}, true, false},
		{"empty", "Areas: []", []string{}, true, false},
		{"missing", "# Design", []string{}, false, false},
		{"escape", "Areas: ../outside", nil, false, true},
		{"absolute", "Areas: /outside", nil, false, true},
		{"malformed", "Areas: bad[", nil, false, true},
		{"partial double star", "Areas: ab**cd", nil, false, true},
		{"empty unit", "| Unit | Size | Areas |\n| --- | --- | --- |\n| U1 | 20 | |\n", nil, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			page := filepath.Join(t.TempDir(), "design.md")
			if err := os.WriteFile(page, []byte(test.text), 0600); err != nil {
				t.Fatal(err)
			}
			got, known, err := DeclaredAreas(test.text)
			if (err != nil) != test.bad || known != test.known || !slices.Equal(got, test.want) {
				t.Fatalf("areas=%v known=%v err=%v", got, known, err)
			}
			if test.name == "union" {
				units, err := DeclaredUnits(page)
				if err != nil || len(units) != 1 || units[0].Lines != 20 {
					t.Fatalf("extra column broke units: %v %v", units, err)
				}
			}
		})
	}
}
