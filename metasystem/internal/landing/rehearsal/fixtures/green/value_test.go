package green

import "testing"

func TestValue(t *testing.T) {
	t.Parallel()
	if Value() != 1 {
		t.Fatal("the fixture value differs")
	}
}
