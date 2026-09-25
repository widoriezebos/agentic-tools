package goal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

func carryBedFor(t *testing.T, base time.Time) (endpoint, other Endpoint, human VerbRequest) {
	t.Helper()
	endpoint, other = fakeGoalEndpointPair(t)
	open := verbReqFor(endpoint, "01J5X00000000000000000C000", "mac-a")
	open.Now = base
	if res, err := Open(open, "g", "Carry one landing past a refusal.", "main", "Land it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	claim := verbReqFor(endpoint, "01J5X00000000000000000C001", "mac-a")
	claim.Now = base
	if res, err := claimApprovedForTest(t, claim, "g", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("claim: %+v %v", res, err)
	}
	human = verbReqFor(endpoint, "01J5X00000000000000000C002", "mac-a")
	human.Now = base.Add(time.Minute)
	human.Actor.Human = "wido"
	return endpoint, other, human
}

func carryVerb(base VerbRequest, ulid string, minutes int) VerbRequest {
	r := base
	r.Ulid = ulid
	r.Now = base.Now.Add(time.Duration(minutes) * time.Minute)
	return r
}

func TestCarryLifecycleLandsThroughTheJournal(t *testing.T) {
	t.Parallel()
	base := time.Now().UTC().Truncate(time.Second)
	endpoint, other, human := carryBedFor(t, base)
	root := endpoint.Root
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
	declareWordHistory(t, endpoint, first, "")
	tree, tip := acceptedTreeForEndpoint(t, endpoint)
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
	open, err := openCarryWordsFor(endpoint, tree, tip, "mac-a", second.Now)
	if err != nil || len(open) != 1 || open[0].History.Opid != first {
		t.Fatalf("open words = %+v, %v", open, err)
	}
	if foreign, err := openCarryWordsFor(endpoint, tree, tip, "mac-b", second.Now); err != nil || len(foreign) != 0 {
		t.Fatalf("another seat's open words = %+v, %v", foreign, err)
	}
	counts, err := countCarriesFor(endpoint, tree, tip, second.Now)
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
	declareWordHistory(t, endpoint, live, "")
	tree, tip = acceptedTreeForEndpoint(t, endpoint)
	consumption, err := carryConsumptionAtFor(endpoint, tree, tip, parsed)
	if err != nil || consumption.Kind != "superseded" || consumption.ID != live {
		t.Fatalf("superseded word's consumption = %+v, %v", consumption, err)
	}
	if replacement, err := CarryWordAt(tree, "g", live); err != nil || replacement.Supersedes != first {
		t.Fatalf("replacement word = %+v, %v", replacement, err)
	}
	if open, err := openCarryWordsFor(endpoint, tree, tip, "mac-a", third.Now); err != nil || len(open) != 1 || open[0].History.Opid != live {
		t.Fatalf("open words after supersede = %+v, %v", open, err)
	}

	// The reservation: refusals at the word, then the row, idempotent on
	// the same seat, abandoned, and reserved again.
	seat := verbReqFor(endpoint, "01J5X00000000000000000C005", "mac-a")
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
	declareWordHistory(t, endpoint, live, "")
	tree, tip = acceptedTreeForEndpoint(t, endpoint)
	if state := CarryReservationAt(tree, "g", live, again.Now); state.State != "open" || state.History.Opid != row {
		t.Fatalf("reservation = %+v", state)
	}
	ledger := tip
	if counts, err := countCarriesFor(endpoint, tree, tip, again.Now); err != nil || counts != (CarryCounts{Open: 1, Inflight: 1}) {
		t.Fatalf("counts with a reservation = %+v, %v", counts, err)
	}
	if debt, found, err := carryDebtAtFor(endpoint, tree, tip, "", again.Now); err != nil || !found || debt.Kind != "inflight" || debt.ID != row {
		t.Fatalf("in-flight debt = %+v %v %v", debt, found, err)
	}
	if _, found, err := carryDebtAtFor(endpoint, tree, tip, live, again.Now); err != nil || found {
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
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
	if state := CarryReservationAt(tree, "g", live, replay.Now); state.State != "abandoned" {
		t.Fatalf("reservation after abandon = %+v", state)
	}
	declareWordHistory(t, endpoint, live, "")
	reserve := carryVerb(seat, "01J5X00000000000000000C009", 4)
	result, row, err = Carrying(reserve, reservation)
	if err != nil || result.Outcome != OutcomeConfirmed || row != reserve.opid() {
		t.Fatalf("carrying after abandon: %+v %q %v", result, row, err)
	}

	// The code lands on origin under the word, the carried intent is
	// journaled beside it, and the completion writes the durable row.
	commit := fakeCodeCarryCommit(t, other, live)
	declareWordHistory(t, endpoint, live, commit)

	var appended []string
	writer := bindCarriedCounselorAppend(nil, func(_ string, goalID string, row HistoryLine, _ time.Time) error {
		appended = append(appended, goalID+":"+row.Opid)
		return nil
	})
	// A nil bind is a no-op: the recorder stays the writer.
	writer = bindCarriedCounselorAppend(writer, nil)
	if err := writer(root, "probe", HistoryLine{Opid: "probe-row"}, base); err != nil || len(appended) != 1 || appended[0] != "probe:probe-row" {
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
	complete.Endpoint.ConfigureCarriedCounselorAppend(writer)
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
	repairEndpoint := endpoint
	repairEndpoint.ConfigureCarriedCounselorAppend(writer)
	if err := RepairCarriedCounselor(repairEndpoint, live, complete.Now); err != nil || len(appended) != 2 {
		t.Fatalf("repair = %v, appended %v", err, appended)
	}
	if err := RepairCarriedCounselor(endpoint, "01J5X00000000000000000C995-mac-a-1a2b3c4d", complete.Now); err == nil || !strings.Contains(err.Error(), "is absent") {
		t.Fatalf("repair of an unknown row = %v", err)
	}

	// The durable row, its obligation, and the readers over it.
	tree, tip = acceptedTreeForEndpoint(t, endpoint)
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
	if consumption, err := carryConsumptionAtFor(endpoint, tree, tip, parsed); err != nil || consumption.Kind != "superseded" {
		t.Fatalf("first word after landing = %+v, %v", consumption, err)
	}
	landedAt, err := time.Parse(time.RFC3339, landed.At)
	if err != nil {
		t.Fatalf("carried row time %q: %v", landed.At, err)
	}
	counts, err = countCarriesFor(endpoint, tree, tip, landedAt)
	if err != nil || counts != (CarryCounts{Today: 1, Debt: 1}) {
		t.Fatalf("counts after landing = %+v, %v", counts, err)
	}
	if debt, found, err := carryDebtAtFor(endpoint, tree, tip, "", complete.Now); err != nil || !found || debt.Kind != "obligation" || debt.Detail != "commit:"+commit {
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
	rebuilt.Endpoint.ConfigureCarriedCounselorAppend(writer)
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
	t.Parallel()
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

func openCarryWordForAbandonTestFor(t *testing.T, base time.Time) (endpoint, other Endpoint, human VerbRequest, proof *humanauthority.Proof, word CarryArgs, ref string) {
	t.Helper()
	endpoint, other, human = carryBedFor(t, base)
	root := endpoint.Root
	configureAbandonFloorTest(t, strings.Repeat("a", 40))
	recordAbandonFloorTestForEndpoint(t, endpoint, "01J5X00000000000000000D000")
	proof = testHumanAuthority(t, root, base)
	word = CarryArgs{
		Goal: "g", Workspace: strings.Repeat("a", 40), Past: "missing-declaration",
		Why: "the bounded exception remains necessary", Expires: base.Add(2 * time.Hour), RaiseFormat: true,
	}
	request := carryVerb(human, "01J5X00000000000000000D010", 1)
	result, err := Carry(request, word, proof)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open carry word: %+v %v", result, err)
	}
	return endpoint, other, human, proof, word, request.opid()
}

func TestAbandonRefusesOpenCarryWords(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	t.Run("missing anchor", func(t *testing.T) {
		t.Parallel()
		endpoint, _ := fakeGoalEndpoint(t)
		ref := "01ARZ3NDEKTSV4RRFFQ69G5FAZ-mac-a-1a2b3c4d"
		tree, file := hclCarryTree("g", ref, base.Add(time.Hour))
		tree.Abandoned = map[string]*GoalFile{}
		declareCarryAt(t, endpoint, "HEAD", ref, "", "")
		err := abandonCarryRefusalFor(endpoint, tree, "HEAD", "g", file, base)
		if err == nil || !strings.Contains(err.Error(), "goal g has open carry word "+ref) || !strings.Contains(err.Error(), "let it expire at "+base.Add(time.Hour).Format(time.RFC3339)) {
			t.Fatalf("missing-anchor refusal = %v", err)
		}
	})
	t.Run("unused word and expiry", func(t *testing.T) {
		t.Parallel()
		endpoint, _, human, _, word, ref := openCarryWordForAbandonTestFor(t, base)
		root := endpoint.Root
		declareWordHistory(t, endpoint, ref, "")
		before, beforeTip := acceptedTreeForEndpoint(t, endpoint)
		request := carryVerb(human, "01J5X00000000000000000D020", 2)
		result, err := Abandon(request, "g", AbandonSpec{Because: "the work is obsolete"}, goalHumanProof(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeRejected {
			t.Fatalf("abandon with open word: %+v %v", result, err)
		}
		want := "goal g has open carry word " + ref
		if !strings.Contains(result.Detail, want) || !strings.Contains(result.Detail, "goal carry --supersede "+ref) || !strings.Contains(result.Detail, word.Expires.Format(time.RFC3339)) {
			t.Fatalf("open-word refusal = %q", result.Detail)
		}
		after, afterTip := acceptedTreeForEndpoint(t, endpoint)
		if beforeTip != afterTip || after.Live["g"] == nil || after.Abandoned["g"] != nil || string(RenderFile(before.Live["g"])) != string(RenderFile(after.Live["g"])) {
			t.Fatal("open-word refusal changed the goal tree")
		}

		expired := carryVerb(human, "01J5X00000000000000000D021", 121)
		expired.Now = word.Expires
		if result, err := Abandon(expired, "g", AbandonSpec{Because: "the unused word expired"}, goalHumanProof(t, root, expired.Now)); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("abandon after word expiry: %+v %v", result, err)
		}
	})

	t.Run("open and closed reservation", func(t *testing.T) {
		t.Parallel()
		endpoint, _, human, _, _, ref := openCarryWordForAbandonTestFor(t, base)
		root := endpoint.Root
		declareWordHistory(t, endpoint, ref, "")
		seat := carryVerb(human, "01J5X00000000000000000D030", 2)
		seat.Actor.Human = ""
		args := CarryingArgs{Goal: "g", ApprovedRef: ref, Workspace: strings.Repeat("a", 40), Project: strings.Repeat("b", 40)}
		result, row, err := Carrying(seat, args)
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open reservation: %+v %q %v", result, row, err)
		}
		declareWordHistory(t, endpoint, ref, "")
		request := carryVerb(human, "01J5X00000000000000000D031", 3)
		result, err = Abandon(request, "g", AbandonSpec{Because: "the work is obsolete"}, goalHumanProof(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "carry reservation "+row+" on goal g is in flight on mac-a") || !strings.Contains(result.Detail, "goal carrying --abandon "+row) {
			t.Fatalf("reservation refusal: %+v %v", result, err)
		}
		closeRequest := carryVerb(seat, "01J5X00000000000000000D032", 4)
		if result, err := AbandonCarrying(closeRequest, "g", row, "the candidate was withdrawn"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("close reservation: %+v %v", result, err)
		}
		declareWordHistory(t, endpoint, ref, "")
		request = carryVerb(human, "01J5X00000000000000000D033", 5)
		result, err = Abandon(request, "g", AbandonSpec{Because: "the work is obsolete"}, goalHumanProof(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "goal g has open carry word "+ref) {
			t.Fatalf("closed reservation hid open word: %+v %v", result, err)
		}
	})

	t.Run("carried commit without ledger row", func(t *testing.T) {
		t.Parallel()
		endpoint, other, human, _, word, ref := openCarryWordForAbandonTestFor(t, base)
		root := endpoint.Root
		commit := fakeCodeCarryCommit(t, other, ref)
		declareWordHistory(t, endpoint, ref, commit)
		request := carryVerb(human, "01J5X00000000000000000D040", 121)
		request.Now = word.Expires.Add(time.Minute)
		result, err := Abandon(request, "g", AbandonSpec{Because: "the landing will not continue"}, goalHumanProof(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "goal g has carried commit "+commit+" without its ledger row") || !strings.Contains(result.Detail, "land.sh --carried "+ref) {
			t.Fatalf("unrecorded carried commit refusal: %+v %v", result, err)
		}
	})

	t.Run("also target is atomic", func(t *testing.T) {
		t.Parallel()
		endpoint, _, human := carryBedFor(t, base)
		root := endpoint.Root
		configureAbandonFloorTest(t, strings.Repeat("a", 40))
		recordAbandonFloorTestForEndpoint(t, endpoint, "01J5X00000000000000000D050")
		open := carryVerb(human, "01J5X00000000000000000D051", 1)
		if result, err := Open(open, "child", "Carry the dependent.", OriginHuman, "Land it."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open child: %+v %v", result, err)
		}
		claim := verbReqFor(endpoint, "01J5X00000000000000000D052", "mac-b")
		claim.Now = base.Add(2 * time.Minute)
		if result, err := claimApprovedForTest(t, claim, "child", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("claim child: %+v %v", result, err)
		}
		proof := testHumanAuthority(t, root, base)
		carryRequest := verbReqFor(endpoint, "01J5X00000000000000000D053", "mac-b")
		carryRequest.Now = base.Add(3 * time.Minute)
		carryRequest.Actor.Human = "wido"
		args := CarryArgs{Goal: "child", Workspace: strings.Repeat("c", 40), Past: "missing-declaration", Why: "the child needs the exception", Expires: base.Add(time.Hour), RaiseFormat: true}
		if result, err := Carry(carryRequest, args, proof); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("carry child: %+v %v", result, err)
		}
		release := carryVerb(claim, "01J5X00000000000000000D054", 4)
		if result, err := Release(release, "child"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("release child: %+v %v", result, err)
		}
		blocked := []string{"g"}
		if result, err := Edit(carryVerb(human, "01J5X00000000000000000D055", 5), "child", EditFields{Blocked: &blocked}); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("block child: %+v %v", result, err)
		}
		declareWordHistory(t, endpoint, carryRequest.opid(), "")
		before, tip := acceptedTreeForEndpoint(t, endpoint)
		request := carryVerb(human, "01J5X00000000000000000D056", 6)
		result, err := Abandon(request, "g", AbandonSpec{Because: "both goals are obsolete", Also: []string{"child"}}, goalHumanProof(t, root, request.Now))
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "goal child has open carry word "+carryRequest.opid()) {
			t.Fatalf("also carry refusal: %+v %v", result, err)
		}
		after, afterTip := acceptedTreeForEndpoint(t, endpoint)
		if tip != afterTip || after.Live["g"] == nil || after.Live["child"] == nil || !contains(after.Live["child"].Blocked, "g") || string(RenderFile(before.Live["child"])) != string(RenderFile(after.Live["child"])) {
			t.Fatal("--also refusal changed a goal or dependent edge")
		}
	})
}

func landedCarryForAbandonTestFor(t *testing.T, base time.Time) (endpoint Endpoint, human VerbRequest, proof *humanauthority.Proof, ref, commit, ordinaryFinding string) {
	t.Helper()
	endpoint, other, human, proof, word, ref := openCarryWordForAbandonTestFor(t, base)
	declareWordHistory(t, endpoint, ref, "")
	reservationRequest := carryVerb(human, "01J5X00000000000000000D059", 2)
	reservationRequest.Actor.Human = ""
	reservationArgs := CarryingArgs{Goal: "g", ApprovedRef: ref, Workspace: word.Workspace, Project: strings.Repeat("b", 40)}
	reservationResult, reservation, reservationErr := Carrying(reservationRequest, reservationArgs)
	if reservationErr != nil || reservationResult.Outcome != OutcomeConfirmed {
		t.Fatalf("open carrying reservation: %+v %q %v", reservationResult, reservation, reservationErr)
	}
	_, ledger := acceptedTreeForEndpoint(t, endpoint)
	commit = fakeCodeCarryCommit(t, other, ref)
	declareWordHistory(t, endpoint, ref, commit)
	seat := carryVerb(human, "01J5X00000000000000000D060", 5)
	seat.Actor.Human = ""
	seat.Endpoint.ConfigureCarriedCounselorAppend(func(string, string, HistoryLine, time.Time) error { return nil })
	args := CarriedArgs{
		Goal: "g", ApprovedRef: ref, Carrying: reservation, Commit: commit, Project: strings.Repeat("b", 40), Workspace: word.Workspace,
		Past: word.Past, Battery: "green", Judge: "base", JudgeDigest: strings.Repeat("d", 64), Ledger: ledger, By: "human:wido",
	}
	if result, err := CarriedFromCommit(seat, args); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record carried landing: %+v %v", result, err)
	}
	ordinaryFinding = "ordinary-review"
	deferRequest := carryVerb(seat, "01J5X00000000000000000D061", 6)
	if result, err := DeferFindings(deferRequest, "g", []ReviewObligation{{Finding: ordinaryFinding, Chain: "critic-chain", Artifact: "commit:" + commit, Test: "pending"}}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("defer ordinary review: %+v %v", result, err)
	}
	return endpoint, human, proof, ref, commit, ordinaryFinding
}

func freezeCarriedGoalForAbandonTestFor(t *testing.T, endpoint Endpoint, now time.Time, ulid string) string {
	t.Helper()
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	file := tree.Live["g"]
	if file == nil || file.StopCapability == nil {
		t.Fatalf("carried goal has no stop capability: %+v", file)
	}
	request := CloseStopRequest{
		VerbRequest: VerbRequest{Endpoint: endpoint, Actor: Actor{Machine: "mac-a", Lineage: "goal-stop-custodian"}, Ulid: ulid, Now: now, ClaimEpoch: file.StopCapability.ClaimEpoch},
		GoalID:      "g", StopID: "stop-g-carried-review", Reason: StopReasonElapsedLimit, Capability: *file.StopCapability,
	}
	if result, err := CloseStop(request); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("freeze carried goal: %+v %v", result, err)
	}
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
	return renderedStopFenceLines(tree.Live["g"])
}

func renderedStopFenceLines(file *GoalFile) string {
	var lines []string
	for _, line := range strings.Split(string(RenderFile(file)), "\n") {
		if strings.HasPrefix(line, "- StopCapability:") || strings.HasPrefix(line, "- StopFence:") {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func openNextCarryGoalForAbandonTestFor(t *testing.T, endpoint Endpoint, now time.Time) {
	t.Helper()
	open := verbReqFor(endpoint, "01J5X00000000000000000D062", "mac-b")
	open.Now = now
	open.Actor.Human = "wido"
	if result, err := Open(open, "next-carry", "Carry after the archived review debt is paid.", OriginHuman, "Issue the next carry word."); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open next carry goal: %+v %v", result, err)
	}
	claim := verbReqFor(endpoint, "01J5X00000000000000000D063", "mac-b")
	claim.Now = now.Add(time.Minute)
	if result, err := claimApprovedForTest(t, claim, "next-carry", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("claim next carry goal: %+v %v", result, err)
	}
}

func TestAbandonKeepsReviewDebtReachable(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)
	endpoint, human, proof, _, commit, ordinaryFinding := landedCarryForAbandonTestFor(t, base)
	root := endpoint.Root
	openNextCarryGoalForAbandonTestFor(t, endpoint, base.Add(13*time.Minute))
	fenceBefore := freezeCarriedGoalForAbandonTestFor(t, endpoint, base.Add(15*time.Minute), "01J5X00000000000000000D064")
	if fenceBefore == "" {
		t.Fatal("frozen carried goal rendered no stop fence lines")
	}
	request := carryVerb(human, "01J5X00000000000000000D070", 15)
	result, err := Abandon(request, "g", AbandonSpec{Because: "the carried work will not continue"}, goalHumanProof(t, root, request.Now))
	if err != nil || result.Outcome != OutcomeConfirmed || !strings.Contains(result.Detail, "finding=carried:"+commit+" chain=human-carried; it still needs discharge") || !strings.Contains(result.Detail, "finding="+ordinaryFinding+" chain=critic-chain; it still needs discharge") {
		t.Fatalf("abandon with review debt: %+v %v", result, err)
	}
	tree, tip := acceptedTreeForEndpoint(t, endpoint)
	if got := renderedStopFenceLines(tree.Abandoned["g"]); got != fenceBefore {
		t.Fatalf("abandon changed frozen stop fence lines:\nbefore: %s\nafter:  %s", fenceBefore, got)
	}
	if debt, found, err := carryDebtAtFor(endpoint, tree, tip, "", request.Now); err != nil || !found || debt.Kind != "obligation" || debt.Goal != "g" || debt.ID != "carried:"+commit {
		t.Fatalf("archived carry debt = %+v %v %v", debt, found, err)
	}
	if counts, err := countCarriesFor(endpoint, tree, tip, request.Now); err != nil || counts.Today != 1 || counts.Debt != 1 {
		t.Fatalf("archived carry counts = %+v %v", counts, err)
	}
	carryArgs := CarryArgs{Goal: "next-carry", Workspace: strings.Repeat("e", 40), Past: "missing-declaration", Why: "the archived carry debt is now the only gate", Expires: request.Now.Add(2 * time.Hour)}
	blockedCarry := verbReqFor(endpoint, "01J5X00000000000000000D065", "mac-b")
	blockedCarry.Now = request.Now
	blockedCarry.Actor.Human = "wido"
	if _, err := Carry(blockedCarry, carryArgs, proof); err == nil {
		t.Fatal("new carry passed while archived human-carried debt was open")
	} else {
		requireCarryAsk(t, err, "carry-debt-unpaid")
	}

	before := string(RenderFile(tree.Abandoned["g"]))
	foreign := carryVerb(human, "01J5X00000000000000000D071", 16)
	foreign.Actor.Human = ""
	if result, err := DischargeReviewObligation(foreign, "g", "carried:"+commit, HumanCarriedChain, "mac-a", "critic-root"); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "requires the human") {
		t.Fatalf("archived discharge without human: %+v %v", result, err)
	}
	wrongChain := carryVerb(human, "01J5X00000000000000000D072", 17)
	if result, err := DischargeReviewObligation(wrongChain, "g", "carried:"+commit, "wrong-chain", "wido", "critic-root"); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "no such obligation") {
		t.Fatalf("archived discharge with wrong chain: %+v %v", result, err)
	}
	wrong := carryVerb(human, "01J5X00000000000000000D073", 18)
	if result, err := DischargeReviewObligation(wrong, "g", "wrong", HumanCarriedChain, "wido", "critic-root"); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "no such obligation") {
		t.Fatalf("archived discharge with wrong finding: %+v %v", result, err)
	}
	if _, err := DischargeReviewObligation(wrong, "g", "carried:"+commit, HumanCarriedChain, "wido", " "); err == nil {
		t.Fatal("archived discharge accepted a blank citation")
	}
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
	if got := string(RenderFile(tree.Abandoned["g"])); got != before {
		t.Fatal("a refused archived discharge changed the record")
	}

	archived := tree.Abandoned["g"]
	stamp := wrong.Now.UTC().Format(time.RFC3339)
	if err := WriteStopBatch(root, StopBatch{
		StopID: archived.StopFence.StopID, GoalID: archived.Id, GoalRevision: archived.StopCapability.Revision,
		FenceEpoch: archived.StopFence.Epoch, CapabilityGeneration: archived.StopCapability.Generation,
		Machine: archived.StopCapability.Machine, ClaimEpoch: archived.StopCapability.ClaimEpoch,
		Reason: archived.StopFence.Reason, State: StopBatchComplete, OpenedAt: stamp, UpdatedAt: stamp, CompletedAt: stamp, Pass: 1,
	}); err != nil {
		t.Fatalf("complete frozen stop batch: %v", err)
	}
	reopen := carryVerb(human, "01J5X00000000000000000D074", 19)
	if result, err := ReopenAbandoned(reopen, "g", goalHumanProof(t, root, reopen.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("reopen with debt: %+v %v", result, err)
	}
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
	if match, err := reviewObligationMatch(tree.Live["g"].ReviewObligations, "carried:"+commit, HumanCarriedChain); err != nil || tree.Live["g"].ReviewObligations[match].State != "open" {
		t.Fatalf("reopen cleared debt: %v", err)
	}
	reabandon := carryVerb(human, "01J5X00000000000000000D075", 20)
	if result, err := Abandon(reabandon, "g", AbandonSpec{Because: "the work remains obsolete"}, goalHumanProof(t, root, reabandon.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("re-abandon with closed carry: %+v %v", result, err)
	}

	discharge := carryVerb(human, "01J5X00000000000000000D076", 21)
	if result, err := DischargeReviewObligation(discharge, "g", "carried:"+commit, HumanCarriedChain, "wido", "critic-root"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("discharge archived carried review: %+v %v", result, err)
	}
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
	carriedMatch, matchErr := reviewObligationMatch(tree.Abandoned["g"].ReviewObligations, "carried:"+commit, HumanCarriedChain)
	if matchErr != nil || tree.Abandoned["g"].ReviewObligations[carriedMatch].State != "discharged" || tree.Abandoned["g"].ReviewObligations[carriedMatch].Test != "critic-root" {
		t.Fatalf("archived discharge evidence = %+v %v", tree.Abandoned["g"].ReviewObligations, matchErr)
	}
	allowedCarry := verbReqFor(endpoint, "01J5X00000000000000000D077", "mac-b")
	allowedCarry.Now = discharge.Now.Add(time.Minute)
	allowedCarry.Actor.Human = "wido"
	carryArgs.Expires = allowedCarry.Now.Add(2 * time.Hour)
	if result, err := Carry(allowedCarry, carryArgs, proof); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("next carry after archived debt discharge: %+v %v", result, err)
	}
	ordinary := carryVerb(human, "01J5X00000000000000000D078", 23)
	if result, err := DischargeReviewObligation(ordinary, "g", ordinaryFinding, "critic-chain", "wido", "successor:test"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("discharge archived ordinary review: %+v %v", result, err)
	}
	tree, tip = acceptedTreeForEndpoint(t, endpoint)
	declareWordHistory(t, endpoint, allowedCarry.opid(), "")
	if debt, found, err := carryDebtAtFor(endpoint, tree, tip, "", ordinary.Now); err != nil || found {
		t.Fatalf("debt after discharge = %+v %v %v", debt, found, err)
	}
}

func TestAbandonedCarriedReviewCanBeWaived(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 13, 0, 0, 0, time.UTC)
	endpoint, human, _, _, commit, ordinaryFinding := landedCarryForAbandonTestFor(t, base)
	root := endpoint.Root
	fenceBefore := freezeCarriedGoalForAbandonTestFor(t, endpoint, base.Add(13*time.Minute), "01J5X00000000000000000D079")
	if fenceBefore == "" {
		t.Fatal("frozen carried goal rendered no stop fence lines")
	}
	abandon := carryVerb(human, "01J5X00000000000000000D080", 13)
	if result, err := Abandon(abandon, "g", AbandonSpec{Because: "the review will not be completed"}, goalHumanProof(t, root, abandon.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon: %+v %v", result, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	before := string(RenderFile(tree.Abandoned["g"]))
	if got := renderedStopFenceLines(tree.Abandoned["g"]); got != fenceBefore {
		t.Fatalf("abandon changed frozen stop fence lines:\nbefore: %s\nafter:  %s", fenceBefore, got)
	}
	waive := carryVerb(human, "01J5X00000000000000000D081", 14)
	if _, err := AcceptedRiskDecision(waive, "g", "carried:"+commit, HumanCarriedChain, "wido", "the bounded delivery risk is accepted", nil); err == nil {
		t.Fatal("archived waiver accepted no proof")
	}
	if _, err := AcceptedRiskDecision(waive, "g", "carried:"+commit, HumanCarriedChain, "wido", " ", goalHumanProof(t, root, waive.Now)); err == nil {
		t.Fatal("archived waiver accepted a blank reason")
	}
	if result, err := AcceptedRiskDecision(waive, "g", ordinaryFinding, "critic-chain", "wido", "the ordinary review is waived", goalHumanProof(t, root, waive.Now)); err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, "not live") {
		t.Fatalf("archived waiver accepted another chain: %+v %v", result, err)
	}
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
	if got := string(RenderFile(tree.Abandoned["g"])); got != before {
		t.Fatal("refused archived waiver changed the record")
	}

	accept := carryVerb(human, "01J5X00000000000000000D082", 15)
	result, err := AcceptedRiskDecision(accept, "g", "carried:"+commit, HumanCarriedChain, "wido", "the bounded delivery risk is accepted", goalHumanProof(t, root, accept.Now))
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("archived waiver: %+v %v", result, err)
	}
	if result, err := AcceptedRiskDecision(accept, "g", "carried:"+commit, HumanCarriedChain, "wido", "the bounded delivery risk is accepted", goalHumanProof(t, root, accept.Now)); err != nil || result.Outcome != OutcomeConfirmed || result.Detail != "idempotent" {
		t.Fatalf("archived waiver replay: %+v %v", result, err)
	}
	resolve := func(repoRoot string) (Endpoint, error) {
		if repoRoot != root {
			return Endpoint{}, os.ErrNotExist
		}
		return endpoint, nil
	}
	if opid, err := acceptedRiskDecisionOpIDWithResolver(root, "g", "carried:"+commit, HumanCarriedChain, accept.Now, resolve); err != nil || opid != accept.opid() {
		t.Fatalf("archived waiver receipt = %q %v", opid, err)
	}
	tree, tip := acceptedTreeForEndpoint(t, endpoint)
	file := tree.Abandoned["g"]
	match, matchErr := reviewObligationMatch(file.ReviewObligations, "carried:"+commit, HumanCarriedChain)
	ordinaryMatch, ordinaryErr := reviewObligationMatch(file.ReviewObligations, ordinaryFinding, "critic-chain")
	if matchErr != nil || ordinaryErr != nil || file.ReviewObligations[match].State != "discharged" || file.ReviewObligations[match].Test != "accepted-risk:"+accept.opid() || file.ReviewObligations[ordinaryMatch].State != "open" || file.State != StateAbandoned {
		t.Fatalf("archived waived obligation = %+v %v", file.ReviewObligations, matchErr)
	}
	if got := renderedStopFenceLines(file); got != fenceBefore {
		t.Fatalf("archived waiver changed frozen stop fence lines:\nbefore: %s\nafter:  %s", fenceBefore, got)
	}
	if debt, found, err := carryDebtAtFor(endpoint, tree, tip, "", accept.Now); err != nil || found {
		t.Fatalf("waived debt remains = %+v %v %v", debt, found, err)
	}
}

func TestAbandonKeepsClosedCarryHistoryVisible(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 15, 0, 0, 0, time.UTC)
	endpoint, human, _, ref, commit, _ := landedCarryForAbandonTestFor(t, base)
	root := endpoint.Root
	abandon := carryVerb(human, "01J5X00000000000000000D090", 7)
	if result, err := Abandon(abandon, "g", AbandonSpec{Because: "retain the carried history"}, goalHumanProof(t, root, abandon.Now)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon carried goal: %+v %v", result, err)
	}
	tree, tip := acceptedTreeForEndpoint(t, endpoint)
	word, err := CarryWordAt(tree, "g", ref)
	if err != nil || word.History.Opid != ref {
		t.Fatalf("archived word = %+v %v", word, err)
	}
	if consumption, err := carryConsumptionAtFor(endpoint, tree, tip, word); err != nil || consumption.Kind != "ledger" {
		t.Fatalf("archived word closer = %+v %v", consumption, err)
	}
	if reservation := CarryReservationAt(tree, "g", ref, abandon.Now); reservation.State != "closed" {
		t.Fatalf("archived reservation state = %+v", reservation)
	}
	if counts, err := countCarriesFor(endpoint, tree, tip, abandon.Now); err != nil || counts.Today != 1 || counts.Debt != 1 || counts.Open != 0 || counts.Inflight != 0 {
		t.Fatalf("archived history counts = %+v %v (commit %s)", counts, err, commit)
	}

	t.Run("superseded words stay closed", func(t *testing.T) {
		t.Parallel()
		endpoint, _, human := carryBedFor(t, base.Add(3*time.Hour))
		root := endpoint.Root
		configureAbandonFloorTest(t, strings.Repeat("a", 40))
		recordAbandonFloorTestForEndpoint(t, endpoint, "01J5X00000000000000000D0A0")
		proof := testHumanAuthority(t, root, base.Add(3*time.Hour))
		firstRequest := carryVerb(human, "01J5X00000000000000000D0A1", 1)
		firstArgs := CarryArgs{Goal: "g", Workspace: strings.Repeat("e", 40), Past: "missing-declaration", Why: "first bounded word", Expires: base.Add(5 * time.Hour), RaiseFormat: true}
		if result, err := Carry(firstRequest, firstArgs, proof); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("first word: %+v %v", result, err)
		}
		open := verbReqFor(endpoint, "01J5X00000000000000000D0A2", "mac-b")
		open.Now = base.Add(3*time.Hour + 2*time.Minute)
		open.Actor.Human = "wido"
		if result, err := Open(open, "replacement", "Hold the replacement word.", OriginHuman, "Use it or let it expire."); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("open replacement: %+v %v", result, err)
		}
		claim := verbReqFor(endpoint, "01J5X00000000000000000D0A3", "mac-b")
		claim.Now = base.Add(3*time.Hour + 3*time.Minute)
		if result, err := claimApprovedForTest(t, claim, "replacement", testBudget()); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("claim replacement: %+v %v", result, err)
		}
		declareWordHistory(t, endpoint, firstRequest.opid(), "")
		secondRequest := verbReqFor(endpoint, "01J5X00000000000000000D0A4", "mac-b")
		secondRequest.Now = base.Add(3*time.Hour + 4*time.Minute)
		secondRequest.Actor.Human = "wido"
		secondArgs := CarryArgs{Goal: "replacement", Workspace: strings.Repeat("f", 40), Past: "missing-declaration", Why: "replacement bounded word", Expires: base.Add(6 * time.Hour), Supersede: firstRequest.opid(), Transfer: true}
		if result, err := Carry(secondRequest, secondArgs, proof); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("replacement word: %+v %v", result, err)
		}
		abandonFirst := carryVerb(human, "01J5X00000000000000000D0A5", 6)
		if result, err := Abandon(abandonFirst, "g", AbandonSpec{Because: "the first word was superseded"}, goalHumanProof(t, root, abandonFirst.Now)); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("abandon superseded word goal: %+v %v", result, err)
		}
		declareWordHistory(t, endpoint, secondRequest.opid(), "")
		abandonSecond := verbReqFor(endpoint, "01J5X00000000000000000D0A6", "mac-b")
		abandonSecond.Now = secondArgs.Expires
		abandonSecond.Actor.Human = "wido"
		if result, err := Abandon(abandonSecond, "replacement", AbandonSpec{Because: "the replacement expired unused"}, goalHumanProof(t, root, abandonSecond.Now)); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("abandon replacement: %+v %v", result, err)
		}
		tree, tip := acceptedTreeForEndpoint(t, endpoint)
		first, firstErr := CarryWordAt(tree, "g", firstRequest.opid())
		second, secondErr := CarryWordAt(tree, "replacement", secondRequest.opid())
		if firstErr != nil || secondErr != nil || first.History.Opid == "" || second.History.Opid == "" {
			t.Fatalf("archived superseded words = %+v/%v %+v/%v", first, firstErr, second, secondErr)
		}
		if consumption, err := carryConsumptionAtFor(endpoint, tree, tip, first); err != nil || consumption.Kind != "superseded" || consumption.ID != secondRequest.opid() {
			t.Fatalf("archived supersede closer = %+v %v", consumption, err)
		}
		if counts, err := countCarriesFor(endpoint, tree, tip, abandonSecond.Now); err != nil || counts.Open != 0 {
			t.Fatalf("archived superseded words reopened the cap: %+v %v", counts, err)
		}
	})
}

func TestDeclareFreeAndLabelDelta(t *testing.T) {
	t.Parallel()
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

	endpoint, _ := fakeGoalEndpoint(t)
	now := time.Now().UTC().Truncate(time.Second)
	open := verbReqFor(endpoint, "01J5X00000000000000000F000", "mac-a")
	open.Now = now
	if res, err := Open(open, "queued-one", "Queued work blocks the declaration.", "main", "Do it."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	declare := verbReqFor(endpoint, "01J5X00000000000000000F001", "mac-a")
	declare.Now = now.Add(time.Minute)
	if res, err := DeclareFree(declare, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeRejected || !strings.Contains(res.Detail, "queued-one is queued") {
		t.Fatalf("declare over a queued goal: %+v %v", res, err)
	}
	park := verbReqFor(endpoint, "01J5X00000000000000000F002", "mac-a")
	park.Now = now.Add(2 * time.Minute)
	if res, err := Park(park, "queued-one", "waiting on the human"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	first := verbReqFor(endpoint, "01J5X00000000000000000F003", "mac-a")
	first.Now = now.Add(3 * time.Minute)
	if res, err := DeclareFree(first, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("declare over parked only: %+v %v", res, err)
	}
	projection, err := Project(endpoint, true, first.Now)
	if err != nil || projection.Tree.Root.Free == nil || projection.Tree.Root.Free.Digest != strings.Repeat("1", 64) || projection.Tree.Root.Free.Origin != "main" {
		t.Fatalf("free record = %+v, %v", projection.Tree.Root, err)
	}
	same := verbReqFor(endpoint, "01J5X00000000000000000F004", "mac-a")
	same.Now = now.Add(4 * time.Minute)
	if res, err := DeclareFree(same, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeAbandoned || !strings.Contains(res.Detail, "already stands") {
		t.Fatalf("declare at the standing digest: %+v %v", res, err)
	}
	if res, err := DeclareFree(first, "main", strings.Repeat("1", 64)); err != nil || res.Outcome != OutcomeConfirmed || res.Detail != "idempotent" {
		t.Fatalf("declare replay of the confirmed entry: %+v %v", res, err)
	}
	renew := verbReqFor(endpoint, "01J5X00000000000000000F005", "mac-a")
	renew.Now = now.Add(5 * time.Minute)
	if res, err := DeclareFree(renew, "main", strings.Repeat("2", 64)); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("declare at a new digest: %+v %v", res, err)
	}
}
