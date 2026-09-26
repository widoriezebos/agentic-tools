package workspace

import (
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uihome"
)

// The private store, as Settings tells a human about it.
//
// D3 of g1-s54: the bounds are said where the human reads. A human who asked
// "what about automatic cleanup, I do not want this to grow and grow until it
// fills up the file system" is answered by three things on one card — where the
// store is, what it holds per workspace, and what it is kept to — and not by a
// design document they would have to go and find.
//
// It is a fact of this server's run like every other field of the workspace
// resource, so it travels on the same read: the About card already has the
// payload, and a second route would be a second fetch for one card.

// Store is what Settings shows about the interface's own private store.
type Store struct {
	// Path is the store's own directory under the account's registry home.
	Path string `json:"path"`
	// Workspaces is what it holds, one entry per workspace, largest first.
	Workspaces []StoreWorkspace `json:"workspaces,omitempty"`
	// Bounds is what the store is kept to, in sentences.
	Bounds []string `json:"bounds,omitempty"`
	// Problem is why there is nothing to report, in the server's own words: an
	// account with no registry home keeps no private store at all, and that is
	// a thing to say rather than an empty card.
	Problem string `json:"problem,omitempty"`
}

// StoreWorkspace is one workspace's share of the store.
type StoreWorkspace struct {
	// Name is the directory's own key: the checkout's last name with a digest
	// of its whole path, which is what makes it that checkout and no other.
	Name string `json:"name"`
	// Size is what it holds, in words.
	Size string `json:"size"`
	// This marks the workspace this server serves, so a human reading several
	// knows which one is in front of them.
	This bool `json:"this,omitempty"`
}

// StoreBounds is the three numbers the store is kept to, as the seat resolved
// them. Zero is a bound that is off, which the words say.
type StoreBounds struct {
	WireMB           int
	ConversationMB   int
	ConversationDays int
}

// DescribeStore measures the store and says what it is kept to.
//
// home is the account's registry home and homeProblem the reason there is none;
// a seat with no home keeps nothing outside its checkout, so it has no store to
// report and says which. The walk is the store's own directory and nothing else.
func DescribeStore(home, homeProblem, checkout string, bounds StoreBounds) Store {
	if homeProblem != "" {
		return Store{Problem: homeProblem}
	}
	store := Store{Path: uihome.Root(home), Bounds: boundWords(bounds)}
	measured, err := uihome.Measure(home)
	if err != nil {
		store.Problem = err.Error()
		return store
	}
	here := uihome.Key(checkout)
	for _, one := range measured {
		store.Workspaces = append(store.Workspaces, StoreWorkspace{
			Name: one.Key, Size: sizeWords(one.Bytes), This: one.Key == here,
		})
	}
	// A store that holds nothing for this workspace yet still says so, because
	// "nothing is kept for this workspace" and "the store could not be read"
	// are different answers and the card must not read as the second.
	if !holds(store.Workspaces, here) {
		store.Workspaces = append(store.Workspaces, StoreWorkspace{
			Name: here, Size: sizeWords(0), This: true,
		})
	}
	return store
}

func holds(workspaces []StoreWorkspace, name string) bool {
	for _, one := range workspaces {
		if one.Name == name {
			return true
		}
	}
	return false
}

// boundWords is the bounds as a human reads them, one sentence per owner, each
// saying what it does or that it is off.
func boundWords(bounds StoreBounds) []string {
	words := []string{}
	if bounds.WireMB > 0 {
		words = append(words, "The Partner's wire journal is rotated at "+
			strconv.Itoa(bounds.WireMB)+" MB, keeping one previous.")
	} else {
		words = append(words, "The Partner's wire journal is not bounded on this seat.")
	}
	switch {
	case bounds.ConversationMB > 0 && bounds.ConversationDays > 0:
		words = append(words, "A conversation is trimmed from its oldest end when it passes "+
			strconv.Itoa(bounds.ConversationMB)+" MB or its oldest message is more than "+
			strconv.Itoa(bounds.ConversationDays)+" days old, and the last 200 messages are always kept.")
	case bounds.ConversationMB > 0:
		words = append(words, "A conversation is trimmed from its oldest end when it passes "+
			strconv.Itoa(bounds.ConversationMB)+" MB, and the last 200 messages are always kept. Nothing is trimmed for age on this seat.")
	case bounds.ConversationDays > 0:
		words = append(words, "A conversation is trimmed from its oldest end when its oldest message is more than "+
			strconv.Itoa(bounds.ConversationDays)+" days old, and the last 200 messages are always kept. Nothing is trimmed for size on this seat.")
	default:
		words = append(words, "Conversations are not bounded on this seat.")
	}
	words = append(words, "Nothing else here is ever removed, and no record of your project is: the records are the memory that survives.")
	return words
}

// sizeWords is a size a human reads. Bytes below a kilobyte are bytes, then
// whole kilobytes, then megabytes with one decimal — the unit the bounds
// themselves are named in, so the number and the bound compare at a glance.
func sizeWords(bytes int64) string {
	switch {
	case bytes < 1024:
		return strconv.FormatInt(bytes, 10) + " bytes"
	case bytes < 1<<20:
		return strconv.FormatInt((bytes+512)/1024, 10) + " KB"
	default:
		return strconv.FormatFloat(float64(bytes)/(1<<20), 'f', 1, 64) + " MB"
	}
}
