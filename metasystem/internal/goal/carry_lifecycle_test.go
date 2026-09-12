package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// carryBed opens and claims one goal on a remote-backed ledger and returns the
// clone that holds the claim, a second clone for code pushes, and the human
// proof the carry word needs.
func carryBed(t *testing.T, base time.Time) (root, other string, human VerbRequest) {
	t.Helper()
	_, root, other = twoClones(t)
	seedLedger(t, root)
	open := verbReq(root, "01J5X00000000000000000C000", "mac-a")
	open.Now = base
	if res, err := Open(open, "g", "Carry one landing past a refusal.", "main", "Land it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	claim := verbReq(root, "01J5X00000000000000000C001", "mac-a")
	claim.Now = base
	if res, err := claimApprovedForTest(t, claim, "g", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	human = verbReq(root, "01J5X00000000000000000C002", "mac-a")
	human.Now = base.Add(time.Minute)
	human.Actor.Human = "wido"
	return root, other, human
}

func carryVerb(base VerbRequest, ulid string, minutes int) VerbRequest {
	r := base
	r.Ulid = ulid
	r.Now = base.Now.Add(time.Duration(minutes) * time.Minute)
	return r
}

func acceptedTree(t *testing.T, root string, now time.Time) (*TreeGoals, string) {
	t.Helper()
	projection, err := Project(endpointFor(root), true, now)
	if err != nil {
		t.Fatal(err)
	}
	return projection.Tree, projection.Tip
}

func TestCarryLifecycleLandsThroughTheJournal(t *testing.T) {
	base := time.Now().UTC().Truncate(time.Second)
	root, other, human := carryBed(t, base)
	proof := testHumanAuthority(t, root, base)
	workspace := strings.Repeat("a", 40)
	word := CarryArgs{Goal: "g", Workspace: workspace, Past: "missing-declaration", Why: "the declaration is a line away", Expires: human.Now.Add(2 * time.Hour)}

	// The terminal word's own guards, before any transaction.
	if _, err := Carry(human, word, nil); err == nil || !strings.Contains(err.Error(), "enrolled human terminal") {
		t.Fatalf("carry without a proof = %v", err)
	}
	if _, err := Carry(human, CarryArgs{Goal: "g"}, proof); err == nil || !strings.Contains(err.Error(), "requires the goal, workspace tree") {
		t.Fatalf("carry without its fields = %v", err)
	}
	expired := word
	expired.Expires = human.Now
	_, err := Carry(human, expired, proof)
	requireCarryAsk(t, err, "carry-word-expired")
	tooLong := word
	tooLong.Expires = human.Now.Add(maximumCarryLife + time.Minute)
	_, err = Carry(human, tooLong, proof)
	requireCarryAsk(t, err, "carry-word-expired")
	notCarryable := word
	notCarryable.Past = "no-such-refusal"
	_, err = Carry(human, notCarryable, proof)
	requireCarryAsk(t, err, "carry-not-carryable")
	_, err = Carry(human, word, proof)
	requireCarryAsk(t, err, "carry-format-required")

	// The first word raises the ledger format; the second is capped; the
	// third supersedes the first.
	raise := word
	raise.RaiseFormat = true
	raiser := carryVerb(human, "01J5X00000000000000000C020", 0)
	result, err := Carry(raiser, raise, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("carry with --raise-format: %+v %v", result, err)
	}
	first := raiser.opid()
	tree, tip := acceptedTree(t, root, raiser.Now)
	if tree.Root.FormatVersion != "2" {
		t.Fatalf("carry left the ledger at format %s", tree.Root.FormatVersion)
	}
	parsed, err := CarryWordAt(tree, "g", first)
	if err != nil || parsed.Workspace != workspace || parsed.Past != "missing-declaration" || !parsed.Expires.Equal(word.Expires) {
		t.Fatalf("carry word = %+v, %v", parsed, err)
	}
	if !CarryWordProven(root, parsed) {
		t.Fatal("the fixture-mode word is not proven")
	}
	second := carryVerb(human, "01J5X00000000000000000C003", 1)
	if result, err := Carry(second, raise, proof); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "--raise-format is only valid") {
		t.Fatalf("second raise = %+v %v", result, err)
	}
	capped := carryVerb(human, "01J5X00000000000000000C021", 1)
	_, err = Carry(capped, word, proof)
	requireCarryAsk(t, err, "carry-cap-reached")
	if !strings.Contains(err.Error(), first+" workspace="+workspace) {
		t.Fatalf("cap ask does not list the open word: %v", err)
	}
	open, err := OpenCarryWords(root, tree, tip, "mac-a", second.Now)
	if err != nil || len(open) != 1 || open[0].History.Opid != first {
		t.Fatalf("open words = %+v, %v", open, err)
	}
	if foreign, err := OpenCarryWords(root, tree, tip, "mac-b", second.Now); err != nil || len(foreign) != 0 {
		t.Fatalf("another seat's open words = %+v, %v", foreign, err)
	}
	counts, err := CountCarries(root, tree, tip, second.Now)
	if err != nil || counts != (CarryCounts{Open: 1}) {
		t.Fatalf("counts = %+v, %v", counts, err)
	}
	supersede := word
	supersede.Supersede = first
	third := carryVerb(human, "01J5X00000000000000000C004", 2)
	result, err = Carry(third, supersede, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("carry --supersede: %+v %v", result, err)
	}
	live := third.opid()
	tree, tip = acceptedTree(t, root, third.Now)
	consumption, err := CarryConsumptionAt(root, tree, tip, parsed)
	if err != nil || consumption.Kind != "superseded" || consumption.ID != live {
		t.Fatalf("superseded word's consumption = %+v, %v", consumption, err)
	}
	if replacement, err := CarryWordAt(tree, "g", live); err != nil || replacement.Supersedes != first {
		t.Fatalf("replacement word = %+v, %v", replacement, err)
	}
	if open, err := OpenCarryWords(root, tree, tip, "mac-a", third.Now); err != nil || len(open) != 1 || open[0].History.Opid != live {
		t.Fatalf("open words after supersede = %+v, %v", open, err)
	}

	// The reservation: refusals at the word, then the row, idempotent on
	// the same seat, abandoned, and reserved again.
	seat := verbReq(root, "01J5X00000000000000000C005", "mac-a")
	seat.Now = third.Now.Add(time.Minute)
	reservation := CarryingArgs{Goal: "g", ApprovedRef: live, Workspace: workspace, Project: strings.Repeat("b", 40)}
	_, _, err = Carrying(seat, CarryingArgs{Goal: "absent", ApprovedRef: live, Workspace: workspace})
	requireCarryAsk(t, err, "carry-goal-not-live")
	stranger := seat
	stranger.Actor.Lineage = "lin-2"
	_, _, err = Carrying(stranger, reservation)
	requireCarryAsk(t, err, "goal-item-not-held")
	_, _, err = Carrying(seat, CarryingArgs{Goal: "g", ApprovedRef: "01J5X00000000000000000C999-mac-a-1a2b3c4d", Workspace: workspace})
	requireCarryAsk(t, err, "carry-word-missing")
	_, _, err = Carrying(seat, CarryingArgs{Goal: "g", ApprovedRef: live, Workspace: strings.Repeat("c", 40)})
	requireCarryAsk(t, err, "carry-tree-mismatch")
	late := seat
	late.Now = word.Expires
	_, _, err = Carrying(late, reservation)
	requireCarryAsk(t, err, "carry-word-expired")
	result, row, err := Carrying(seat, reservation)
	if err != nil || result.Outcome != OutcomeConfirmed || row != seat.opid() {
		t.Fatalf("carrying: %+v %q %v", result, row, err)
	}
	again := carryVerb(seat, "01J5X00000000000000000C006", 1)
	if result, sameRow, err := Carrying(again, reservation); err != nil || result.Outcome != OutcomeConfirmed || sameRow != row {
		t.Fatalf("carrying again on the same seat: %+v %q %v", result, sameRow, err)
	}
	tree, tip = acceptedTree(t, root, again.Now)
	if state := CarryReservationAt(tree, "g", live, again.Now); state.State != "open" || state.History.Opid != row {
		t.Fatalf("reservation = %+v", state)
	}
	ledger := tip
	if counts, err := CountCarries(root, tree, tip, again.Now); err != nil || counts != (CarryCounts{Open: 1, Inflight: 1}) {
		t.Fatalf("counts with a reservation = %+v, %v", counts, err)
	}
	if debt, found, err := CarryDebtAt(root, tree, tip, "", again.Now); err != nil || !found || debt.Kind != "inflight" || debt.ID != row {
		t.Fatalf("in-flight debt = %+v %v %v", debt, found, err)
	}
	if _, found, err := CarryDebtAt(root, tree, tip, live, again.Now); err != nil || found {
		t.Fatalf("the reservation's own word counts as debt: %v %v", found, err)
	}
	_, _, err = Carrying(again, CarryingArgs{Goal: "g", ApprovedRef: live, Carrying: "01J5X00000000000000000C998-mac-a-1a2b3c4d", Commit: strings.Repeat("d", 40), Workspace: workspace})
	requireCarryAsk(t, err, "carry-debt-unpaid")
	if _, _, err := Carrying(again, CarryingArgs{Goal: "g", ApprovedRef: live, Carrying: row, Commit: strings.Repeat("d", 40), Workspace: strings.Repeat("c", 40)}); err == nil || !strings.Contains(err.Error(), "reservation workspace differs") {
		t.Fatalf("carried intent with another workspace = %v", err)
	}
	abandon := carryVerb(seat, "01J5X00000000000000000C007", 2)
	if _, err := AbandonCarrying(abandon, "g", "01J5X00000000000000000C997-mac-a-1a2b3c4d", "typo"); err == nil || !strings.Contains(err.Error(), "is missing") {
		t.Fatalf("abandon of an unknown row = %v", err)
	}
	if _, err := AbandonCarrying(abandon, "absent", row, "typo"); err == nil || !strings.Contains(err.Error(), "is not live") {
		t.Fatalf("abandon on an absent goal = %v", err)
	}
	otherSeat := abandon
	otherSeat.Actor.Machine = "mac-b"
	_, err = AbandonCarrying(otherSeat, "g", row, "not mine")
	requireCarryAsk(t, err, "carry-seat-mismatch")
	if result, err := AbandonCarrying(abandon, "g", row, "the tree moved"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon: %+v %v", result, err)
	}
	replay := carryVerb(seat, "01J5X00000000000000000C008", 3)
	if result, err := AbandonCarrying(replay, "g", row, "the tree moved"); err != nil || result.Outcome != OutcomeConfirmed || result.Detail != "idempotent" {
		t.Fatalf("abandon replay: %+v %v", result, err)
	}
	tree, _ = acceptedTree(t, root, replay.Now)
	if state := CarryReservationAt(tree, "g", live, replay.Now); state.State != "abandoned" {
		t.Fatalf("reservation after abandon = %+v", state)
	}
	reserve := carryVerb(seat, "01J5X00000000000000000C009", 4)
	result, row, err = Carrying(reserve, reservation)
	if err != nil || result.Outcome != OutcomeConfirmed || row != reserve.opid() {
		t.Fatalf("carrying after abandon: %+v %q %v", result, row, err)
	}

	// The code lands on origin under the word, the carried intent is
	// journaled beside it, and the completion writes the durable row.
	mustGit(t, other, "pull", "-q", "origin", "main")
	if err := os.WriteFile(filepath.Join(other, "carried.txt"), []byte("carried\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, other, "add", "carried.txt")
	mustGit(t, other, "commit", "-qm", "carried change", "-m", "Carry: "+live)
	commit := mustGit(t, other, "rev-parse", "HEAD")
	mustGit(t, other, "push", "-q", "origin", "main")

	previous := carriedCounselorAppend
	t.Cleanup(func() { carriedCounselorAppend = previous })
	var appended []string
	BindCarriedCounselorAppend(func(_ string, goalID string, row HistoryLine, _ time.Time) error {
		appended = append(appended, goalID+":"+row.Opid)
		return nil
	})
	// A nil bind is a no-op: the recorder stays the writer.
	BindCarriedCounselorAppend(nil)
	if err := carriedCounselorAppend(root, "probe", HistoryLine{Opid: "probe-row"}, base); err != nil || len(appended) != 1 || appended[0] != "probe:probe-row" {
		t.Fatalf("nil bind replaced the writer: %v %v", err, appended)
	}
	appended = nil
	intent := CarryingArgs{Goal: "g", ApprovedRef: live, Carrying: row, Commit: commit, Project: strings.Repeat("b", 40), Workspace: workspace,
		Past: "missing-declaration", Battery: "green", Judge: "base", JudgeDigest: strings.Repeat("e", 64), Ledger: ledger, By: "human:wido", OwnerPID: int64(os.Getpid())}
	journal := carryVerb(seat, "01J5X00000000000000000C010", 5)
	result, entry, err := Carrying(journal, intent)
	if err != nil || result.Outcome != OutcomeConfirmed || entry != journal.opid() || result.Detail != "carrying="+entry {
		t.Fatalf("carried intent: %+v %q %v", result, entry, err)
	}
	if _, _, err := Carrying(carryVerb(seat, "01J5X00000000000000000C011", 6), intent); err == nil || !strings.Contains(err.Error(), "is in flight") {
		t.Fatalf("second carried intent beside a live owner = %v", err)
	}
	bogus := carryVerb(seat, "01J5X00000000000000000C012", 7)
	if _, err := Carried(bogus, "01J5X00000000000000000C996-mac-a-1a2b3c4d"); err == nil || !strings.Contains(err.Error(), "no journal entry") {
		t.Fatalf("carried of an unknown entry = %v", err)
	}
	complete := carryVerb(seat, "01J5X00000000000000000C013", 8)
	result, err = Carried(complete, entry)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("carried: %+v %v", result, err)
	}
	if len(appended) != 1 || appended[0] != "g:"+entry {
		t.Fatalf("counselor append = %v", appended)
	}
	if result, err := Carried(complete, entry); err != nil || result.Outcome != OutcomeConfirmed || result.Detail != "idempotent" {
		t.Fatalf("carried replay: %+v %v", result, err)
	}
	if err := RepairCarriedCounselor(endpointFor(root), live, complete.Now); err != nil || len(appended) != 2 {
		t.Fatalf("repair = %v, appended %v", err, appended)
	}
	if err := RepairCarriedCounselor(endpointFor(root), "01J5X00000000000000000C995-mac-a-1a2b3c4d", complete.Now); err == nil || !strings.Contains(err.Error(), "is absent") {
		t.Fatalf("repair of an unknown row = %v", err)
	}

	// The durable row, its obligation, and the readers over it.
	tree, tip = acceptedTree(t, root, complete.Now)
	file := tree.Live["g"]
	if LandedCarryCount(file) != 1 || file.BudgetExceptions != 1 {
		t.Fatalf("landed count %d exceptions %d", LandedCarryCount(file), file.BudgetExceptions)
	}
	goalID, landed, ok := carriedRow(tree, live)
	if !ok || goalID != "g" || !strings.HasPrefix(landed.Reason, "landed commit="+commit+" ") {
		t.Fatalf("carried row = %q %+v %v", goalID, landed, ok)
	}
	if len(file.ReviewObligations) != 1 || file.ReviewObligations[0].Finding != "carried:"+commit || file.ReviewObligations[0].Artifact != "commit:"+commit {
		t.Fatalf("obligation = %+v", file.ReviewObligations)
	}
	if state := CarryReservationAt(tree, "g", live, complete.Now); state.State != "closed" {
		t.Fatalf("reservation after landing = %+v", state)
	}
	if consumption, err := CarryConsumptionAt(root, tree, tip, parsed); err != nil || consumption.Kind != "superseded" {
		t.Fatalf("first word after landing = %+v, %v", consumption, err)
	}
	landedAt, err := time.Parse(time.RFC3339, landed.At)
	if err != nil {
		t.Fatalf("carried row time %q: %v", landed.At, err)
	}
	counts, err = CountCarries(root, tree, tip, landedAt)
	if err != nil || counts != (CarryCounts{Today: 1, Debt: 1}) {
		t.Fatalf("counts after landing = %+v, %v", counts, err)
	}
	if debt, found, err := CarryDebtAt(root, tree, tip, "", complete.Now); err != nil || !found || debt.Kind != "obligation" || debt.Detail != "commit:"+commit {
		t.Fatalf("obligation debt = %+v %v %v", debt, found, err)
	}
	if !strings.Contains(carryDebtText(CarryDebt{Kind: "obligation", Goal: "g", ID: "f", Detail: "commit:x"}), "open obligation f") ||
		!strings.Contains(carryDebtText(CarryDebt{Kind: "unrecorded", ID: live, Detail: commit}), "without its ledger row") ||
		carryDebtText(CarryDebt{Kind: "other"}) != "carry debt is unpaid" {
		t.Fatal("debt texts")
	}
	fourth := carryVerb(human, "01J5X00000000000000000C014", 9)
	_, err = Carry(fourth, word, proof)
	requireCarryAsk(t, err, "carry-debt-unpaid")

	// The rebuilt record replays only when every field agrees.
	record := CarriedArgs{Goal: "g", ApprovedRef: live, Carrying: row, Commit: commit, Project: strings.Repeat("b", 40), Workspace: workspace,
		Past: "missing-declaration", Battery: "green", Judge: "base", JudgeDigest: strings.Repeat("e", 64), Ledger: ledger, By: "human:wido"}
	rebuilt := carryVerb(seat, "01J5X00000000000000000C016", 11)
	if result, err := CarriedFromCommit(rebuilt, record); err != nil || result.Outcome != OutcomeConfirmed || result.Detail != "idempotent" {
		t.Fatalf("carried from commit replay: %+v %v", result, err)
	}
	changed := record
	changed.Battery = "red"
	if result, err := CarriedFromCommit(carryVerb(seat, "01J5X00000000000000000C017", 12), changed); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "battery differs") {
		t.Fatalf("carried from commit with a changed field: %+v %v", result, err)
	}
	if _, err := CarriedFromCommit(VerbRequest{Endpoint: Endpoint{Root: root, Remote: "local"}}, record); err == nil {
		t.Fatal("carried from commit accepted a local endpoint")
	}
}

func TestCarryReadersAndJournalGuards(t *testing.T) {
	if !ValidCarryToken("carry workspace="+strings.Repeat("a", 40)+" goal=g past=missing-declaration") ||
		ValidCarryToken("carry workspace=short goal=g past=x") || ValidCarryToken(" carry workspace="+strings.Repeat("a", 40)+" goal=g past=x") {
		t.Fatal("carry token shape")
	}
	if reasonField("open workspace=w project=p expires=e", "project") != "p" || reasonField("open workspace=w", "expires") != "" {
		t.Fatal("reason field")
	}
	if ids := SortedGoalIds(map[string]*GoalFile{"b": nil, "a": nil}); len(ids) != 2 || ids[0] != "a" || ids[1] != "b" {
		t.Fatalf("sorted ids = %v", ids)
	}
	if LandedCarryCount(nil) != 0 {
		t.Fatal("nil file landed count")
	}
	now := time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)
	opid := "01ARZ3NDEKTSV4RRFFQ69G5FAY-mac-a-1a2b3c4d"
	tree, _ := hclCarryTree("g", opid, now.Add(time.Hour))
	if _, err := CarryWordAt(nil, "g", opid); err == nil || !strings.Contains(err.Error(), "tree is absent") {
		t.Fatalf("absent tree = %v", err)
	}
	if _, err := CarryWordAt(tree, "h", opid); err == nil || !strings.Contains(err.Error(), "goal h is absent") {
		t.Fatalf("absent goal = %v", err)
	}
	if _, err := CarryWordAt(tree, "g", "01ARZ3NDEKTSV4RRFFQ69G5FAX-mac-a-1a2b3c4d"); err == nil || !strings.Contains(err.Error(), "is missing on goal g") {
		t.Fatalf("missing word = %v", err)
	}
	if word, err := CarryWordAt(tree, "g", opid); err != nil || word.Past != "missing-declaration" {
		t.Fatalf("word = %+v, %v", word, err)
	}
	if list := carryWordList(carryWords(tree)); !strings.Contains(list, opid+" workspace="+strings.Repeat("a", 40)+" expires=") {
		t.Fatalf("word list = %q", list)
	}
	if !CarryableName("", "missing-declaration") || CarryableName("", "group:none") || CarryableName("", "nothing") {
		t.Fatal("carryable names")
	}
	module := filepath.Join("..", "..")
	if !CarryableName(module, "group:section/go-engine-gate") || CarryableName(module, "group:no-such-group") || CarryableName(t.TempDir(), "group:section/go-engine-gate") {
		t.Fatal("group names")
	}

	root := t.TempDir()
	entryOpid := "01J5X00000000000000000C900-mac-a-1a2b3c4d"
	intent := Intent{Verb: "carried", Targets: []string{"g"}, Args: map[string]string{"approvedRef": "w"}}
	if _, err := CreateCarryingEntry(root, "", "mac-a", "lin-a", intent, 0); err == nil {
		t.Fatal("carrying entry without an opid")
	}
	if _, err := CreateCarryingEntry(root, entryOpid, "mac-a", "lin-a", Intent{Verb: "claim"}, 0); err == nil {
		t.Fatal("carrying entry with a non-carried intent")
	}
	if _, err := CreateCarryingEntry(root, entryOpid, "mac-a", "lin-a", intent, int64(os.Getpid())); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateCarryingEntry(root, entryOpid, "mac-a", "lin-a", intent, int64(os.Getpid())); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate entry = %v", err)
	}
	endpoint := Endpoint{Root: root, Remote: "origin", Branch: "refs/heads/main"}
	if _, err := CompleteEntry(endpoint, PublishRequest{Opid: "01J5X00000000000000000C901-mac-a-1a2b3c4d"}); err == nil || !strings.Contains(err.Error(), "no journal entry") {
		t.Fatalf("complete of an unknown entry = %v", err)
	}
	different := Intent{Verb: "carried", Targets: []string{"g"}, Args: map[string]string{"approvedRef": "other"}}
	if _, err := CompleteEntry(endpoint, PublishRequest{Opid: entryOpid, Intent: different}); err == nil || !strings.Contains(err.Error(), "intent differs") {
		t.Fatalf("complete with another intent = %v", err)
	}
	if intentsEqual(intent, Intent{Verb: "carried", Targets: []string{"h"}, Args: intent.Args}) ||
		intentsEqual(intent, Intent{Verb: "carried", Targets: []string{"g"}, Args: map[string]string{"approvedRef": "w", "extra": "x"}}) ||
		!intentsEqual(intent, Intent{Verb: "carried", Targets: []string{"g"}, Args: map[string]string{"approvedRef": "w"}}) ||
		intentsEqual(Intent{Deltas: []FieldDelta{{Field: "a"}}}, Intent{Deltas: []FieldDelta{{Field: "b"}}}) {
		t.Fatal("intent equality")
	}
	if err := MarkTerminal(root, entryOpid, OutcomeConfirmed, "landed"); err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteEntry(endpoint, PublishRequest{Opid: entryOpid, Intent: intent}); err == nil || !strings.Contains(err.Error(), "not created or pushed") {
		t.Fatalf("complete of a terminal entry = %v", err)
	}
	if _, err := TakeOverForCompletion(root, entryOpid); err == nil || !strings.Contains(err.Error(), "is terminal") {
		t.Fatalf("takeover of a terminal entry = %v", err)
	}
}

func TestDeclareFreeAndLabelDelta(t *testing.T) {
	labels, err := ApplyLabelDelta([]string{"b", "a"}, []string{"c"}, []string{"a"})
	if err != nil || strings.Join(labels, ",") != "b,c" {
		t.Fatalf("label delta = %v, %v", labels, err)
	}
	if _, err := ApplyLabelDelta(nil, []string{"x"}, []string{"x"}); err == nil || !strings.Contains(err.Error(), "cannot be both") {
		t.Fatalf("contradictory delta = %v", err)
	}
	if _, err := ApplyLabelDelta(nil, []string{"Not Valid"}, nil); err == nil {
		t.Fatal("invalid added label")
	}
	if _, err := ApplyLabelDelta(nil, nil, []string{"Not Valid"}); err == nil {
		t.Fatal("invalid removed label")
	}

	_, root, _ := twoClones(t)
	seedLedger(t, root)
	now := time.Now().UTC().Truncate(time.Second)
	open := verbReq(root, "01J5X00000000000000000F000", "mac-a")
	open.Now = now
	if res, err := Open(open, "queued-one", "Queued work blocks the declaration.", "main", "Do it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	declare := verbReq(root, "01J5X00000000000000000F001", "mac-a")
	declare.Now = now.Add(time.Minute)
	if res, err := DeclareFree(declare, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "queued-one is queued") {
		t.Fatalf("declare over a queued goal: %+v %v", res, err)
	}
	park := verbReq(root, "01J5X00000000000000000F002", "mac-a")
	park.Now = now.Add(2 * time.Minute)
	if res, err := Park(park, "queued-one", "waiting on the human"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	first := verbReq(root, "01J5X00000000000000000F003", "mac-a")
	first.Now = now.Add(3 * time.Minute)
	if res, err := DeclareFree(first, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("declare over parked only: %+v %v", res, err)
	}
	projection, err := Project(endpointFor(root), true, first.Now)
	if err != nil || projection.Tree.Root.Free == nil || projection.Tree.Root.Free.Digest != strings.Repeat("1", 64) || projection.Tree.Root.Free.Origin != "main" {
		t.Fatalf("free record = %+v, %v", projection.Tree.Root, err)
	}
	same := verbReq(root, "01J5X00000000000000000F004", "mac-a")
	same.Now = now.Add(4 * time.Minute)
	if res, err := DeclareFree(same, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeAbandoned || !strings.Contains(res.Detail, "already stands") {
		t.Fatalf("declare at the standing digest: %+v %v", res, err)
	}
	if res, err := DeclareFree(first, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeConfirmed || res.Detail != "idempotent" {
		t.Fatalf("declare replay of the confirmed entry: %+v %v", res, err)
	}
	renew := verbReq(root, "01J5X00000000000000000F005", "mac-a")
	renew.Now = now.Add(5 * time.Minute)
	if res, err := DeclareFree(renew, "main", strings.Repeat("2", 64)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("declare at a new digest: %+v %v", res, err)
	}
}
