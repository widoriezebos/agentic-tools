package pathpattern

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGLEPathGrammarAndExpansion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, name := range []string{"cmd/landing_batch.go", "cmd/landing_batch_land.go", "cmd/other.go"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	pattern, err := Parse(`./cmd\landing_batch*.go`)
	if err != nil {
		t.Fatal(err)
	}
	if pattern.String() != "cmd/landing_batch*.go" || !pattern.Match("cmd/landing_batch_land.go") || pattern.Match("cmd/other.go") {
		t.Fatalf("normalized wildcard pattern = %s", pattern.String())
	}
	got, err := pattern.Expand(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"cmd/landing_batch.go", "cmd/landing_batch_land.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expanded = %v, want %v", got, want)
	}
	optional, err := Parse("cmd/future?.go")
	if err != nil {
		t.Fatal(err)
	}
	got, err = optional.Expand(root)
	if err != nil || len(got) != 0 {
		t.Fatalf("optional match = %v, %v", got, err)
	}
	if !optional.Match("cmd/future1.go") {
		t.Fatal("future addition would be missed")
	}
}

func TestGLEPathRejectsUnsupportedAndEscapingPatterns(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"../outside", "a/../outside", "/absolute", `C:\outside`, "a/**/file", "a/***.go", "a/[ab].go", "a/{a,b}.go", ""} {
		if _, err := Parse(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
	pattern, err := Parse("src/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if pattern.Match("src/../outside.go") || pattern.Match("src/nested/a.go") {
		t.Fatal("pattern escaped its component")
	}
}

func TestGLEPathOverlapAndSymlinkEscape(t *testing.T) {
	t.Parallel()
	for _, pair := range [][2]string{{"src/*.go", "src/a?.go"}, {"reports/**", "reports"}, {"src/a*", "src/*b"}} {
		a, _ := Parse(pair[0])
		b, _ := Parse(pair[1])
		if !a.Overlaps(b) {
			t.Errorf("missed overlap %v", pair)
		}
	}
	a, _ := Parse("src/*.go")
	b, _ := Parse("src/*.txt")
	if a.Overlaps(b) {
		t.Fatal("disjoint suffixes overlap")
	}
	root := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	p, _ := Parse("linked/*.go")
	if _, err := p.Expand(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlink parent = %v", err)
	}
	p, _ = Parse("*/file.go")
	if _, err := p.Expand(root); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("wildcard symlink parent = %v", err)
	}
}

func TestGLEPathUnicodeOverlap(t *testing.T) {
	t.Parallel()
	wild, _ := Parse("src/?.txt")
	accented, _ := Parse("src/é.txt")
	if !wild.Match("src/é.txt") || !wild.Overlaps(accented) {
		t.Fatal("Unicode character was matched but overlap was missed")
	}
	disjoint, _ := Parse("src/💡.go")
	if wild.Overlaps(disjoint) {
		t.Fatal("disjoint Unicode suffixes overlap")
	}
}

func TestGLEPathVersionedManifestLiteral(t *testing.T) {
	t.Parallel()
	entry := EncodeLiteral("cmd/[literal].go")
	if matched, err := MatchManifestEntry(entry, "cmd/[literal].go"); err != nil || !matched {
		t.Fatalf("literal did not match itself: %v, %v", matched, err)
	}
	if matched, err := MatchManifestEntry(entry, "cmd/aliteral.go"); err != nil || matched {
		t.Fatalf("literal matched a different file: %v, %v", matched, err)
	}
	if matched, err := MatchManifestEntry("cmd/[literal].go", "cmd/[literal].go"); err != nil || !matched {
		t.Fatalf("legacy literal did not match: %v, %v", matched, err)
	}
	if _, err := MatchManifestEntry("\x00literal:v2:AA", "cmd/[literal].go"); err == nil {
		t.Fatal("unknown manifest version accepted")
	}
}
