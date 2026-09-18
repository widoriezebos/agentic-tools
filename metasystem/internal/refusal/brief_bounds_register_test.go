package refusal

import "testing"

func TestBriefBoundsRefusalRegistration(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		want := Row{Code: "BRIEF_BOUNDS_INVALID", Owner: "internal/dispatch", Site: "brief.go:57", Shape: Question}
		var got Row
		for _, row := range Rows {
			if row.Code == "BRIEF_BOUNDS_INVALID" {
				got = row
			}
		}
		if got != want {
			t.Fatalf("row = %+v, want %+v", got, want)
		}
	})
	t.Run("bounds-unreadable", func(t *testing.T) {
		want := Row{Code: "BRIEF_BOUNDS_UNREADABLE", Owner: "internal/validate", Site: "brief_bounds_source.go:19", Shape: Question}
		var got Row
		for _, row := range Rows {
			if row.Code == "BRIEF_BOUNDS_UNREADABLE" {
				got = row
			}
		}
		if got != want {
			t.Fatalf("row = %+v, want %+v", got, want)
		}
	})
}
