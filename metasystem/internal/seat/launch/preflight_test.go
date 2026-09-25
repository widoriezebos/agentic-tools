package launch

// The preflight's refusals, by name, and the two exemptions a resume gets.

import (
	"os"
	"path/filepath"
	"testing"
)

func askFor(machine, where string) Request {
	return Request{Machine: machine, From: fromRoot, Destination: where, Word: humanWord, ReviewBy: reviewBy}
}

func refusalOf(t *testing.T, err error) *Refusal {
	t.Helper()
	refusal, named := err.(*Refusal)
	if !named {
		t.Fatalf("error = %v, want a named refusal", err)
	}
	return refusal
}

func TestAnUnpublishableNicknameIsRefusedByName(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"m1/f", ".", "..", "a..b", "m1 f", ""} {
		err := Preflight(askFor(name, filepath.Join(t.TempDir(), "clone")), Facts{})
		if got := refusalOf(t, err); got.Code != CodeNicknameInvalid {
			t.Fatalf("%q = %s, want %s", name, got.Code, CodeNicknameInvalid)
		}
	}
}

func TestThisSeatsOwnNicknameIsRefused(t *testing.T) {
	t.Parallel()
	err := Preflight(askFor("m1u", filepath.Join(t.TempDir(), "clone")), Facts{This: "m1u"})
	if got := refusalOf(t, err); got.Code != CodeNicknameInvalid {
		t.Fatalf("code = %s, want %s", got.Code, CodeNicknameInvalid)
	}
}

func TestANicknameTheFleetAlreadyCarriesIsRefused(t *testing.T) {
	t.Parallel()
	err := Preflight(askFor("m1e", filepath.Join(t.TempDir(), "clone")), Facts{This: "m1u", Taken: []string{"m1b", "m1e"}})
	if got := refusalOf(t, err); got.Code != CodeNicknameTaken {
		t.Fatalf("code = %s, want %s", got.Code, CodeNicknameTaken)
	}
}

func TestAResumeKeepsTheNicknameItsOwnLaunchSet(t *testing.T) {
	t.Parallel()
	where := filepath.Join(t.TempDir(), "clone")
	if err := os.Mkdir(where, 0o755); err != nil {
		t.Fatal(err)
	}
	asked := askFor("m1f", where)
	asked.Resume = launchID
	facts := Facts{This: "m1u", Taken: []string{"m1f"}, Created: Created{Destination: true, Nickname: true}}
	if err := Preflight(asked, facts); err != nil {
		t.Fatalf("a resume was refused its own nickname: %v", err)
	}
}

func TestADestinationThatIsAlreadyThereIsRefused(t *testing.T) {
	t.Parallel()
	where := filepath.Join(t.TempDir(), "clone")
	if err := os.Mkdir(where, 0o755); err != nil {
		t.Fatal(err)
	}
	err := Preflight(askFor("m1f", where), Facts{})
	if got := refusalOf(t, err); got.Code != CodeDestinationExists {
		t.Fatalf("code = %s, want %s", got.Code, CodeDestinationExists)
	}
}

func TestADestinationInsideAnotherCheckoutIsRefused(t *testing.T) {
	t.Parallel()
	checkout := t.TempDir()
	if err := os.Mkdir(filepath.Join(checkout, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := Preflight(askFor("m1f", filepath.Join(checkout, "beneath", "clone")), Facts{})
	if got := refusalOf(t, err); got.Code != CodeDestinationInsideACheckout {
		t.Fatalf("code = %s, want %s", got.Code, CodeDestinationInsideACheckout)
	}
}

func TestAResumesOwnDestinationIsNotRefusedForBeingACheckout(t *testing.T) {
	t.Parallel()
	where := filepath.Join(t.TempDir(), "clone")
	if err := os.MkdirAll(filepath.Join(where, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	asked := askFor("m1f", where)
	asked.Resume = launchID
	if err := Preflight(asked, Facts{Created: Created{Destination: true}}); err != nil {
		t.Fatalf("a resume was refused the clone it made: %v", err)
	}
}

func TestADestinationThatIsNotAbsoluteIsRefused(t *testing.T) {
	t.Parallel()
	err := Preflight(askFor("m1f", "agentic-tools-m1f"), Facts{})
	if got := refusalOf(t, err); got.Code != CodeDestinationExists {
		t.Fatalf("code = %s, want %s", got.Code, CodeDestinationExists)
	}
}

// The review date is judged against the day the CLIENT was on, which is the
// one rule both boundaries use: a browser and its server are on different
// dates for several hours of every day, and the date a human answered is the
// one that was on their screen.
func TestAReviewDateIsJudgedAgainstTheDayTheCallerWasOn(t *testing.T) {
	t.Parallel()
	const today = "2026-09-25"
	if !ReviewDateBefore("2026-09-24", today) {
		t.Fatal("yesterday reads as a review that is not yet due")
	}
	// The server is one day more permissive than the sheet on purpose: a
	// request written a minute before midnight is not refused for arriving a
	// minute after it.
	if ReviewDateBefore(today, today) {
		t.Fatal("the caller's own day reads as a date before it")
	}
	if ReviewDateBefore("2026-10-02", today) {
		t.Fatal("a week out reads as a review already overdue")
	}
	if ReviewDateBefore("not a date", today) || ReviewDateBefore("2026-09-24", "not a date") {
		t.Fatal("a malformed date is the word pair validator's refusal and not this one")
	}
	if !ValidDay(today) || ValidDay("25/09/2026") || ValidDay("") {
		t.Fatal("a day is one plain YYYY-MM-DD and nothing else")
	}
}

func TestTheDefaultDestinationIsNamedForTheRemotesRepository(t *testing.T) {
	t.Parallel()
	for url, repository := range map[string]string{
		"https://github.com/widoriezebos/agentic-tools.git": "agentic-tools",
		"https://github.com/widoriezebos/agentic-tools":     "agentic-tools",
		"git@github.com:widoriezebos/other-project.git":     "other-project",
		"/srv/mirrors/agentic-tools.git/":                   "agentic-tools",
		"":                                                  "",
	} {
		if got := RepositoryName(url); got != repository {
			t.Fatalf("RepositoryName(%q) = %q, want %q", url, got, repository)
		}
	}
	if got := DefaultDestination("/w/agentic-tools", "agentic-tools", "m1f"); got != "/w/agentic-tools-m1f" {
		t.Fatalf("default destination = %q", got)
	}
}

func TestSiblingsAreTheClonesBesideThisCheckout(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	for _, name := range []string{"agentic-tools", "agentic-tools-m1b", "agentic-tools-m1c", "something-else", "agentic-tools-"} {
		if err := os.Mkdir(filepath.Join(parent, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := Siblings(parent, "agentic-tools")
	if len(got) != 2 || got[0] != "m1b" || got[1] != "m1c" {
		t.Fatalf("siblings = %v, want m1b and m1c", got)
	}
}
