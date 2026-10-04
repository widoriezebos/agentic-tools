package plain

import (
	"errors"
	"slices"
	"testing"
)

func TestUnitsOnMainCountsDistinctTrailerUnits(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		trailers string
		count    int
		fails    bool
	}{
		{"goal/a+b\ngoal/a\nother/z\ngoal/c\n", 3, false},
		{"", 0, false},
		{"goal/a+a", 0, true},
		{"goal/a/b", 0, true},
		{"goal/a b", 0, true},
		{"goal/", 0, true},
	} {
		count, err := unitsOnMain("/lane", "head", "goal", func(dir string, args ...string) (string, error) {
			want := []string{"log", "--format=%(trailers:key=Goal-Unit,valueonly)", "--fixed-strings", "--grep=Goal-Unit: goal/", "head"}
			if dir != "/lane" || !slices.Equal(args, want) {
				t.Fatalf("trailer read: %s %q", dir, args)
			}
			return test.trailers, nil
		})
		if count != test.count || (err != nil) != test.fails {
			t.Errorf("trailers %q: count=%d err=%v", test.trailers, count, err)
		}
	}
	broken := errors.New("log unreadable")
	if _, err := unitsOnMain("/lane", "head", "goal", func(string, ...string) (string, error) { return "", broken }); !errors.Is(err, broken) {
		t.Fatalf("read error lost: %v", err)
	}
}
