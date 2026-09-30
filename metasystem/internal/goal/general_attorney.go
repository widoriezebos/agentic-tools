package goal

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// GeneralGrant is what a person grants with metasystem grant add --acts
// everything: one checkout of one machine, the lease-holder lineage at the
// grant, and an end on a whole minute.
type GeneralGrant struct {
	Machine  string
	Checkout string
	Lineage  string
	Until    time.Time
}

// GrantGeneral records a general power of attorney. Only the person's own
// proof at the enrolled terminal grants it: never the helm's yield, never a
// proof a grant admitted, never a relayed, channel or session word. A live
// general grant for the same checkout is revoked in the same commit; the
// same grant again is a repeat.
func GrantGeneral(r VerbRequest, proof *humanauthority.Proof, g GeneralGrant) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("a general power of attorney is a person's act and requires --by from an authorized human boundary")
	}
	if proof == nil || proof.Helm != nil || !proof.EnrolledTerminalFor(r.Endpoint.Root) {
		return PublishResult{}, errors.New("a general grant is added by the person at their own terminal, not at the helm or under a grant")
	}
	for label, value := range map[string]string{"machine": g.Machine, "checkout": g.Checkout, "lineage": g.Lineage} {
		if value == "" || strings.ContainsAny(value, " \t\r\n") {
			return PublishResult{}, fmt.Errorf("a general power of attorney needs a %s with no whitespace, not %q", label, value)
		}
	}
	if !filepath.IsAbs(g.Checkout) {
		return PublishResult{}, fmt.Errorf("a general power of attorney names its checkout by an absolute path, not %s", g.Checkout)
	}
	until := g.Until.UTC()
	if !until.Equal(until.Truncate(time.Minute)) {
		return PublishResult{}, fmt.Errorf("a general power of attorney ends on a whole minute")
	}
	since := r.Now.UTC().Truncate(time.Second)
	if !until.After(since) || until.Sub(since) > GeneralAttorneyMax {
		return PublishResult{}, fmt.Errorf("a general power of attorney ends after now and at most one week later (by %s)", since.Add(GeneralAttorneyMax).Format(time.RFC3339))
	}
	entry := PowerOfAttorneyEntry{ID: r.opid(), By: r.Actor.historyActor(), Verbs: []string{GeneralAct},
		For: g.Machine, Checkout: g.Checkout, Lineage: g.Lineage, Since: since.Format(time.RFC3339), Until: until.Format(time.RFC3339)}
	if within, why := entry.WithinBounds(); !within {
		return PublishResult{}, fmt.Errorf("a general power of attorney outside its bounds: %s", why)
	}
	reason := "verbs=" + GeneralAct + " for=" + g.Machine + " checkout=" + g.Checkout + " until=" + entry.Until
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "grant", Args: intentArgs(r, map[string]string{
			"verbs": GeneralAct, "for": g.Machine, "checkout": g.Checkout, "lineage": g.Lineage, "until": entry.Until,
		})},
		Message: "goal grant everything",
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			if _, exists := rootAttorney(t.Root, r.opid()); exists {
				return nil, AlreadyApplied{}
			}
			for i := range t.Root.PowerOfAttorney {
				standing := &t.Root.PowerOfAttorney[i]
				if !standing.General() || standing.Checkout != g.Checkout || standing.For != g.Machine {
					continue
				}
				if live, _ := standing.LiveAt(r.Now); !live {
					continue
				}
				if standing.By == entry.By && standing.Lineage == entry.Lineage && standing.Until == entry.Until {
					return nil, AlreadyHolds{Reason: fmt.Sprintf("power of attorney %s already grants exactly this (until %s)", standing.ID, standing.Until)}
				}
				standing.Revoked = r.stamp()
				standing.RevokedBy = r.Actor.historyActor()
			}
			t.Root.PowerOfAttorney = append(t.Root.PowerOfAttorney, entry)
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{At: r.stamp(), Opid: r.opid(), Verb: "grant", Actor: r.Actor.historyActor(), Keep: -1, Reason: reason})
			recordSessionAuthority(&t.Root.History[len(t.Root.History)-1], proof)
			return []Change{{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)}}, nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	})
}

// GeneralGrantsAt reads the power-of-attorney entries of the accepted
// ledger's root record only: the accepted tip and one file, memoized per
// tip within the process. No accepted ledger reads as no entries.
func GeneralGrantsAt(e Endpoint) (string, []PowerOfAttorneyEntry, error) {
	tip, present, err := e.repository().Accepted()
	if err != nil {
		return "", nil, err
	}
	if !present {
		return "", nil, nil
	}
	entries, err := rootEntriesAt(e, tip)
	return tip, entries, err
}

var rootEntryMemo sync.Map // root+"\x00"+tip -> []PowerOfAttorneyEntry

func rootEntriesAt(e Endpoint, tip string) ([]PowerOfAttorneyEntry, error) {
	key := e.Root + "\x00" + tip
	if cached, ok := rootEntryMemo.Load(key); ok {
		return cached.([]PowerOfAttorneyEntry), nil
	}
	files, err := e.repository().Files(tip, goalsPrefix+"backlog.md")
	if err != nil {
		return nil, err
	}
	content, ok := files[goalsPrefix+"backlog.md"]
	if !ok {
		return nil, fmt.Errorf("no root record at %s", short(tip))
	}
	record, problems := ParseRoot(content)
	if len(problems) > 0 {
		return nil, fmt.Errorf("the root record at %s does not parse: %s", short(tip), problems[0])
	}
	rootEntryMemo.Store(key, record.PowerOfAttorney)
	return record.PowerOfAttorney, nil
}

// attorneyBinding ties one act's endpoint to the general grant that
// answered its person check, so each of its mutations re-checks the grant.
type attorneyBinding struct {
	id  string
	now func() (time.Time, error)
}

// WithAttorneyEffect returns a copy of the endpoint whose publishes re-check
// grant id against now at the tip each commits on. The binding lives with
// the request that carries this endpoint and ends with it.
func (e Endpoint) WithAttorneyEffect(id string, now func() (time.Time, error)) Endpoint {
	e.attorney = &attorneyBinding{id: id, now: now}
	return e
}

// AttorneyEffect names the grant this endpoint's publishes re-check, if any.
func (e Endpoint) AttorneyEffect() string {
	if e.attorney == nil {
		return ""
	}
	return e.attorney.id
}

// guardAttorneyEffect wraps a mutation with the bound grant's re-check.
func guardAttorneyEffect(e Endpoint, mutate func(string) ([]Change, error)) func(string) ([]Change, error) {
	if e.attorney == nil {
		return mutate
	}
	binding := *e.attorney
	return func(tip string) ([]Change, error) {
		entries, err := rootEntriesAt(e, tip)
		if err != nil {
			return nil, err
		}
		var entry *PowerOfAttorneyEntry
		for i := range entries {
			if entries[i].ID == binding.id {
				entry = &entries[i]
			}
		}
		if entry == nil {
			return nil, fmt.Errorf("power of attorney %s is not recorded at the tip this act lands on", binding.id)
		}
		now, err := binding.now()
		if err != nil {
			return nil, fmt.Errorf("power of attorney %s: the clock is unreadable: %w", binding.id, err)
		}
		if live, why := entry.LiveAt(now); !live {
			return nil, fmt.Errorf("power of attorney %s is not live at the act: %s", binding.id, why)
		}
		return mutate(tip)
	}
}
