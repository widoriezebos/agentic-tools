package goal

import "testing"

func TestAreasOverlap(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		a, b         string
		overlap, bad bool
	}{
		{"future/new.go", "future/new.go", true, false},
		{"src/*.go", "src/*.md", true, false},
		{"src/**/a.go", "src/[ab]?.md", true, false},
		{"src/", "src/nested/file.go", true, false},
		{"**/a.go", "elsewhere/b.go", true, false},
		{"src/a*", "other/a*", false, false},
		{".", "src/a", true, false},
		{"src/[", "src/a", false, true},
	} {
		t.Run(test.a+test.b, func(t *testing.T) {
			t.Parallel()
			overlap, err := AreasOverlap([]string{test.a}, []string{test.b})
			if (err != nil) != test.bad || (overlap != nil) != test.overlap {
				t.Fatalf("overlap=%+v err=%v", overlap, err)
			}
		})
	}
}
