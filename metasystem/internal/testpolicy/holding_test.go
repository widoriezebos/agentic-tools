package testpolicy

import (
	"reflect"
	"testing"
)

func TestHoldingSurfacesAnyFileAndFallback(t *testing.T) {
	t.Parallel()
	contract := Contract{Fallback: "residual", Surfaces: []Surface{
		{ID: "first", Paths: []string{"unit/one.go", "unit/*.md"}},
		{ID: "second", Paths: []string{"unit/two.go", "unit/*.md"}},
		{ID: "residual"},
	}}
	for _, tc := range []struct {
		name        string
		files, want []string
		fallback    string
	}{
		{"one file", []string{"unit/one.go"}, []string{"first"}, "residual"},
		{"different files", []string{"unit/two.go", "unit/one.go", "unit/one.go"}, []string{"first", "second"}, "residual"},
		{"shared file", []string{"unit/readme.md"}, []string{"first", "second"}, "residual"},
		{"fallback", []string{"unit/one.go", "elsewhere/file.go"}, []string{"first", "residual"}, "residual"},
		{"no fallback", []string{"elsewhere/file.go"}, []string{}, ""},
		{"no files", nil, []string{}, "residual"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := contract
			c.Fallback = tc.fallback
			if got := HoldingSurfaces(c, tc.files); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("holding surfaces = %v; want %v", got, tc.want)
			}
		})
	}
}
