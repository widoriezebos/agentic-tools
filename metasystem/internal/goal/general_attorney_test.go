package goal

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func generalEntry(since, until time.Time) PowerOfAttorneyEntry {
	return PowerOfAttorneyEntry{ID: Opid("01J5X00000000000000000PG00", "mac-a", "lin-1"), By: "human:Wido",
		Verbs: []string{GeneralAct}, For: "mac-a", Checkout: "/seat/checkout", Lineage: "lin-main",
		Since: since.UTC().Format(time.RFC3339), Until: until.UTC().Format(time.RFC3339)}
}

// W2: a general entry round-trips through the root record, its grammar is
// closed in both directions, and it is live only in [since, until).
func TestGeneralAttorneyEntryGrammarAndLife(t *testing.T) {
	t.Parallel()
	since := time.Date(2026, 9, 30, 7, 0, 0, 0, time.UTC)
	entry := generalEntry(since, since.Add(24*time.Hour))
	root := &RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "1", SyncMode: SyncLocal, Revision: 1,
		PowerOfAttorney: []PowerOfAttorneyEntry{entry}}
	rendered := RenderRoot(root)
	want := "- " + entry.ID + " by=human:Wido verbs=everything for=mac-a checkout=/seat/checkout lineage=lin-main since=2026-09-30T07:00:00Z until=2026-10-01T07:00:00Z"
	if !strings.Contains(string(rendered), want) {
		t.Fatalf("the general entry renders %q:\n%s", want, rendered)
	}
	parsed, problems := ParseRoot(rendered)
	if len(problems) != 0 || len(parsed.PowerOfAttorney) != 1 || string(RenderRoot(parsed)) != string(rendered) {
		t.Fatalf("round trip: %v %+v", problems, parsed)
	}
	if !parsed.PowerOfAttorney[0].General() {
		t.Fatal("the parsed entry is general")
	}
	id := entry.ID
	for label, line := range map[string]string{
		"tiers on general":      id + " by=human:Wido tiers=1 verbs=everything for=m checkout=/c lineage=l since=2026-09-30T07:00:00Z until=2026-10-01T07:00:00Z",
		"expires on general":    id + " by=human:Wido verbs=everything for=m checkout=/c lineage=l since=2026-09-30T07:00:00Z until=2026-10-01T07:00:00Z expires=2026-10-01",
		"missing until":         id + " by=human:Wido verbs=everything for=m checkout=/c lineage=l since=2026-09-30T07:00:00Z",
		"missing checkout":      id + " by=human:Wido verbs=everything for=m lineage=l since=2026-09-30T07:00:00Z until=2026-10-01T07:00:00Z",
		"until on scoped":       id + " by=human:Wido tiers=1 verbs=approve since=2026-09-30T07:00:00Z expires=2026-10-01 until=2026-10-01T07:00:00Z",
		"lineage on scoped":     id + " by=human:Wido tiers=1 verbs=approve since=2026-09-30T07:00:00Z expires=2026-10-01 lineage=l",
		"everything with other": id + " by=human:Wido verbs=everything,approve for=m checkout=/c lineage=l since=2026-09-30T07:00:00Z until=2026-10-01T07:00:00Z",
	} {
		if _, err := parseAttorneyEntry(line); err == nil {
			t.Errorf("%s: parsed, want refused", label)
		}
	}
	if _, err := parseAttorneyEntry(id + " by=human:Wido tiers=1 verbs=approve since=2026-09-30T07:00:00Z expires=2026-10-01"); err != nil {
		t.Fatalf("the scoped grammar is unchanged: %v", err)
	}

	until := since.Add(24 * time.Hour)
	if live, why := entry.LiveAt(until.Add(-time.Nanosecond)); !live {
		t.Fatalf("live just before until: %s", why)
	}
	for label, now := range map[string]time.Time{"at until": until, "before since": since.Add(-time.Second), "zero": {}} {
		if live, _ := entry.LiveAt(now); live {
			t.Errorf("%s: live, want not", label)
		}
	}
	revoked := entry
	revoked.Revoked, revoked.RevokedBy = since.Add(time.Hour).Format(time.RFC3339), "human:Wido"
	if live, why := revoked.LiveAt(since.Add(2 * time.Hour)); live || !strings.Contains(why, "revoked") {
		t.Fatalf("revoked: %v %s", live, why)
	}
	for label, bad := range map[string]PowerOfAttorneyEntry{
		"over a week":       generalEntry(since, since.Add(168*time.Hour+time.Minute)),
		"not a minute":      generalEntry(since, since.Add(time.Hour+time.Second)),
		"relative checkout": func() PowerOfAttorneyEntry { e := entry; e.Checkout = "seat"; return e }(),
		"until before":      generalEntry(since, since.Add(-time.Hour)),
	} {
		if within, _ := bad.WithinBounds(); within {
			t.Errorf("%s: within bounds, want not", label)
		}
	}
	if within, why := generalEntry(since, since.Add(168*time.Hour)).WithinBounds(); !within {
		t.Fatalf("exactly one week is the bound: %s", why)
	}
}

func generalGrantBed(t *testing.T) (Endpoint, VerbRequest, *humanauthority.Proof) {
	t.Helper()
	endpoint := attorneyBed(t)
	human := attorneyReq(endpoint, 40, "mac-a")
	human.Actor.Human = "Wido"
	return endpoint, human, testHumanAuthority(t, endpoint.Root, human.Now)
}

// W5 and W8 at the owner: the person's own proof records one general grant
// per checkout; a repeat holds, a new end replaces, a proof admitted by the
// helm or a grant never grants, and a scoped --under never uses it.
func TestGrantGeneralRecordsOneGrantPerCheckout(t *testing.T) {
	t.Parallel()
	endpoint, human, proof := generalGrantBed(t)
	until := human.Now.Add(24 * time.Hour).Truncate(time.Minute)
	grant := GeneralGrant{Machine: "mac-a", Checkout: "/seat/checkout", Lineage: "lin-main", Until: until}
	res, err := GrantGeneral(human, proof, grant)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	tree, err := loadTreeFor(endpoint, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := rootAttorney(tree.Root, human.opid())
	if !ok || !entry.General() || entry.Checkout != "/seat/checkout" || entry.Lineage != "lin-main" || entry.For != "mac-a" || entry.Until != until.UTC().Format(time.RFC3339) {
		t.Fatalf("the entry: %+v", entry)
	}
	if last := tree.Root.History[len(tree.Root.History)-1]; last.Verb != "grant" || last.Actor != "human:Wido" || !strings.Contains(last.Reason, "verbs=everything") {
		t.Fatalf("the history line: %+v", last)
	}
	if _, err := resolveAttorneyForEndpoint(endpoint, entry.ID, "approve", human.Now); err == nil {
		t.Fatal("a scoped act under --under never uses a general entry")
	}

	repeat := withUlid(human, 41)
	if res, err := GrantGeneral(repeat, proof, grant); err != nil || res.Outcome == OutcomeConfirmed {
		t.Fatalf("the same grant again holds: %+v %v", res, err)
	}
	longer := withUlid(human, 42)
	grant.Until = until.Add(time.Hour)
	res, err = GrantGeneral(longer, proof, grant)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("a new end: %+v %v", res, err)
	}
	if tree, err = loadTreeFor(endpoint, res.Tip); err != nil {
		t.Fatal(err)
	}
	old, _ := rootAttorney(tree.Root, entry.ID)
	if old.Revoked == "" || old.RevokedBy != "human:Wido" {
		t.Fatalf("the replaced grant is revoked in the same commit: %+v", old)
	}

	for label, bad := range map[string]*humanauthority.Proof{
		"helm": func() *humanauthority.Proof { p := *proof; p.Helm = &humanauthority.HelmGrant{By: "Wido"}; return &p }(),
		"attorney": func() *humanauthority.Proof {
			p := *proof
			p.Helm = &humanauthority.HelmGrant{By: "Wido", Grant: entry.ID}
			return &p
		}(),
		"none": nil,
	} {
		res, err := GrantGeneral(withUlid(human, 43), bad, grant)
		expectRefusal(t, label, res, err, "enrolled terminal")
	}
	scoped := withUlid(human, 44)
	attorneyProof := *proof
	attorneyProof.Helm = &humanauthority.HelmGrant{By: "Wido", Grant: entry.ID}
	res, err = Grant(scoped, &attorneyProof, []uint8{1}, []string{"approve"}, human.Now.Format("2006-01-02"))
	expectRefusal(t, "scoped grant under a grant", res, err, "enrolled terminal")

	for label, g := range map[string]GeneralGrant{
		"past":         {Machine: "mac-a", Checkout: "/seat/checkout", Lineage: "l", Until: human.Now.Add(-time.Minute)},
		"over a week":  {Machine: "mac-a", Checkout: "/seat/checkout", Lineage: "l", Until: human.Now.Add(169 * time.Hour).Truncate(time.Minute)},
		"spaced path":  {Machine: "mac-a", Checkout: "/seat/my checkout", Lineage: "l", Until: until},
		"relative":     {Machine: "mac-a", Checkout: "seat", Lineage: "l", Until: until},
		"no lineage":   {Machine: "mac-a", Checkout: "/seat/checkout", Until: until},
		"not a minute": {Machine: "mac-a", Checkout: "/seat/checkout", Lineage: "l", Until: until.Add(time.Second)},
	} {
		if res, err := GrantGeneral(withUlid(human, 45), proof, g); err == nil && res.Outcome == OutcomeConfirmed {
			t.Errorf("%s: granted, want refused", label)
		}
	}
}

// W6 at the owner: a process bound to a grant re-checks it at the tip every
// mutation lands on, against its own clock, so a revoke or an expiry that
// lands after admission refuses the act with nothing written.
func TestAnActUnderAGrantRechecksItAtTheEffect(t *testing.T) {
	t.Parallel()
	endpoint, human, proof := generalGrantBed(t)
	until := human.Now.Add(time.Hour).Truncate(time.Minute)
	res, err := GrantGeneral(human, proof, GeneralGrant{Machine: "mac-a", Checkout: "/seat/checkout", Lineage: "lin-main", Until: until})
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	id := human.opid()
	clock := human.Now.Add(time.Minute)
	bound := endpoint.WithAttorneyEffect(id, func() (time.Time, error) { return clock, nil })
	if bound.AttorneyEffect() != id || endpoint.AttorneyEffect() != "" {
		t.Fatalf("the binding belongs to the bound endpoint only: %q %q", bound.AttorneyEffect(), endpoint.AttorneyEffect())
	}
	open := func(on Endpoint, n int, name string) (PublishResult, error) {
		request := asPerson(t, endpoint.Root, attorneyReq(endpoint, n, "mac-a"))
		request.Endpoint = on
		return OpenRisked(request, name, "Work "+name+".", OriginHuman, "Do it.", nil, nil,
			RiskRecord{Severity: 1, Novelty: 1, Exposure: 1, Accumulation: 1, Basis: "routine"}, 0, "", nil, nil)
	}
	if res, err := open(bound, 50, "while-live"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("an act while the grant is live: %+v %v", res, err)
	}
	clock = until
	res, err = open(bound, 51, "after-expiry")
	expectRefusal(t, "expired at the effect", res, err, "is not live")
	if res, err := open(endpoint, 55, "other-act"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("another act on the same root is not bound: %+v %v", res, err)
	}

	clock = human.Now.Add(2 * time.Minute)
	if res, err := Revoke(withUlid(human, 52), proof, id); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("revoke: %+v %v", res, err)
	}
	res, err = open(bound, 53, "after-revoke")
	expectRefusal(t, "revoked at the effect", res, err, "revoked")
	res, err = open(endpoint.WithAttorneyEffect("01M-unknown", func() (time.Time, error) { return clock, nil }), 54, "unknown-grant")
	expectRefusal(t, "unknown grant", res, err, "not recorded")
}
