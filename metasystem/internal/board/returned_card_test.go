package board

import (
	"testing"
)

func TestLiveOrReturnedCardPrefersLiveClaim(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	returned := Card{Goal: "goal", Seat: Seat{Machine: "old", Installation: "/old/metasystem"}, Stage: StageReturned}
	if err := WriteAt(home, returned); err != nil {
		t.Fatal(err)
	}
	if card, ok := LiveOrReturnedCard(home, "goal"); !ok || card.Stage != StageReturned {
		t.Fatalf("return cannot rejoin: %+v %v", card, ok)
	}
	live := returned
	live.Seat, live.Stage = Seat{Machine: "new", Installation: "/new/metasystem"}, StageLandReady
	if err := WriteAt(home, live); err != nil {
		t.Fatal(err)
	}
	if card, ok := LiveOrReturnedCard(home, "goal"); !ok || card.Seat != live.Seat || card.Stage != live.Stage {
		t.Fatalf("old return hid live claim: %+v %v", card, ok)
	}
	if card, ok := LiveCard(home, "goal"); !ok || card.Seat != live.Seat {
		t.Fatalf("live lookup changed: %+v %v", card, ok)
	}
}
