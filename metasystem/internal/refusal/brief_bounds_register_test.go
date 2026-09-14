package refusal

import "testing"

func TestBriefBoundsRefusalRegistration(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		want := Row{Code: "BRIEF_BOUNDS_INVALID", Owner: "internal/dispatch", Site: "brief.go:68", Shape: Question}
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
}
