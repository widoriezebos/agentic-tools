package goal

import (
	"errors"
	"testing"
	"time"
)

type unfetchedRepository struct{ Repository }

func (unfetchedRepository) Accepted() (string, bool, error) { return "", false, nil }

// EM-25: a checkout that has not fetched the ledger was told "no accepted
// tree; the first fetch or the migration bootstraps it". The error says what
// is missing and the command that fetches it, and callers can tell it apart.
func TestProjectOfAnUnfetchedLedgerNamesTheFetch(t *testing.T) {
	t.Parallel()
	_, err := Project(Endpoint{Root: t.TempDir(), Repository: unfetchedRepository{}}, false, time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrLedgerNotFetched) {
		t.Fatalf("unfetched ledger = %v, want ErrLedgerNotFetched", err)
	}
	if want := "this checkout has not fetched the goal ledger yet; metasystem goal list --fetch fetches it"; err.Error() != want {
		t.Fatalf("unfetched ledger = %q, want %q", err, want)
	}
}
