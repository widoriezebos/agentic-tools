package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/stickies"
)

// fixtureStoreHome is the registry home this fixture invents, which is the
// private store Settings measures and names. It is beside the fixture checkout
// and never the account's own, for the reason the notepad's own comment gives.
func fixtureStoreHome(checkout string) string {
	return filepath.Join(filepath.Dir(checkout), filepath.Base(checkout)+"-notepad", ".metasystem")
}

// The fixture's notepad: six stickies, for every handle this fixture can act
// as, under a home of its own.
//
// The home is a temporary directory and NEVER the account's own. The store
// resolves the real one from the registry's home, and a walkthrough that wrote
// there would put fixture notes into a human's actual notepad; so this one is
// handed a home beside the fixture checkout and prints it, exactly as the
// fixture prints the checkout it invents.
//
// Six, because that is what the panel has to be able to show: one about a goal
// and one about a document, so the chips and their links are both on screen;
// one about several things at once; one about nothing at all, which is the
// note J4's own example is made of and the one that only reaches the Partner
// through the panel; and two already struck off, so "Done (2)" has something
// behind its disclosure.
func fixtureStickies(checkout string, handles []string, now time.Time) *stickies.Store {
	home := fixtureStoreHome(checkout)
	if err := os.MkdirAll(filepath.Dir(home), 0o700); err != nil {
		log.Fatalf("cannot make the walkthrough notepad: %v", err)
	}
	written := 0
	// The clock steps back a minute per sticky and then forward again, so the
	// six land in a known order with ages a human can read on the cards rather
	// than six stickies all written at the same instant.
	store := stickies.New(home, checkout, func() time.Time {
		written++
		return now.Add(-time.Duration(len(fixtureNotes)*len(handles)-written) * 11 * time.Minute)
	})
	fmt.Println("notepad " + store.File())
	seen := map[string]bool{}
	for _, human := range handles {
		if seen[human] {
			continue
		}
		seen[human] = true
		for _, note := range fixtureNotes {
			list, err := store.Add(human, note.text, note.about)
			if err != nil {
				log.Fatalf("cannot plant the walkthrough notepad: %v", err)
			}
			if !note.done {
				continue
			}
			struck := true
			id := list.Stickies[0].ID
			if _, err := store.Edit(human, id, nil, nil, &struck); err != nil {
				log.Fatalf("cannot strike off a walkthrough sticky: %v", err)
			}
		}
	}
	return store
}

// fixtureNotes is what the six say, in the order they were written.
var fixtureNotes = []struct {
	text  string
	about []stickies.About
	done  bool
}{
	{text: "ask Sol whether the retry cap covers a stopped turn",
		about: []stickies.About{{Kind: stickies.KindGoal, ID: "reading-pane"}}},
	{text: "the launch sheet's wording is off — \"temporary word\" reads like a placeholder",
		about: []stickies.About{{Kind: stickies.KindRecord, ID: "plans/designs/shell.md"}}},
	{text: "check g1-s13 tomorrow; the goal page still opens on the wrong tab for me",
		about: []stickies.About{
			{Kind: stickies.KindGoal, ID: "g1-s13"},
			{Kind: stickies.KindRecord, ID: "plans/designs/shell.md"},
		}},
	// The one about nothing on any page. It is the note Astra's F2 is about:
	// without the panel in the capture it would never reach the Partner.
	{text: "the Fleet page feels cramped at phone width — look again with fresh eyes"},
	{text: "read the rulings register end to end before the next retro", done: true},
	{text: "reply to the steward's message about the undelivered notification", done: true},
}
