package httpd

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// backlogPath is the backlog resource, matched exactly: what lies beneath it
// belongs to no resource, so it is a 404 like any other unserved path under a
// reserved prefix.
const backlogPath = "/api/backlog"

// backlogSchemaVersion is the shape of the resource a reader parses.
const backlogSchemaVersion = 1

type backlogPayload struct {
	SchemaVersion int                `json:"schemaVersion"`
	ObservedAt    string             `json:"observedAt"`
	Ledger        ledgerPayload      `json:"ledger"`
	Admission     admissionPayload   `json:"admission"`
	WorkingTree   workingTreePayload `json:"workingTree"`
	// Authority says, before a human drags anything, whether a drop on this
	// board would act. It is the server's one boot-time observation, fixed
	// for its life, so every request gets the same answer.
	Authority authorityPayload `json:"authority"`
	// BudgetDefaults is the project's budget law by tier, which is where the
	// approval sheet prefills from. An empty map is a project whose law
	// could not be read, and the sheet then asks for all five limits.
	BudgetDefaults map[string]goalbudget.Budget `json:"budgetDefaults"`
	Counts         map[backlog.Lane]int         `json:"counts"`
	Draft          backlog.DraftGap             `json:"draft"`
	Rows           []boardRow                   `json:"rows"`
	Closed         []boardRow                   `json:"closed"`
}

// boardRow is the interface's own row: the backlog projection's row exactly
// as it stands, with the one fact this layer joins to it.
//
// The backlog package is not taught about presence. A row says who claimed a
// goal, and whether that machine has been heard from lately is a second
// reading — of the presence copy this interface fetched for itself — that
// belongs to whoever holds both. That is this layer, and the join is made
// from the same observation the rows were projected from, so the holder and
// the row can never be from two commits.
type boardRow struct {
	backlog.Row
	Holder *holderPayload `json:"holder,omitempty"`
}

// holderPayload is the standing of the machine that holds a claimed row. It
// is a flag and never an act: the goal stays claimed, and the words name what
// a human can do at a terminal.
type holderPayload struct {
	Machine  string `json:"machine"`
	Standing string `json:"standing"`
	Since    string `json:"since"`
	Flag     string `json:"flag"`
}

// wrapRows carries the projection's rows into the interface's own shape, with
// no holder on any of them: the join is a second step, because a build that
// cannot read presence still serves a board.
func wrapRows(rows []backlog.Row) []boardRow {
	wrapped := make([]boardRow, 0, len(rows))
	for _, row := range rows {
		wrapped = append(wrapped, boardRow{Row: row})
	}
	return wrapped
}

// plainRows is the projection's rows again, for a composer that reads the
// board rather than the page: Overview fills its own holders in its own
// composition.
func plainRows(rows []boardRow) []backlog.Row {
	plain := make([]backlog.Row, 0, len(rows))
	for _, row := range rows {
		plain = append(plain, row.Row)
	}
	return plain
}

// authorityPayload is what the board is told about this server's standing. It
// never carries the proof: a proof is an in-process observation, and nothing
// outside this process can be handed one.
type authorityPayload struct {
	Proven bool   `json:"proven"`
	Human  string `json:"human"`
	Reason string `json:"reason"`
}

type ledgerPayload struct {
	State string `json:"state"`
	Tip   string `json:"tip"`
	// CommittedAt is the accepted commit's committer time, which is the age
	// of the tree; nothing records when this clone accepted it. It is a fact
	// the chip reports and never a warning: a repository nobody has committed
	// to since breakfast is a quiet repository, not a broken one.
	CommittedAt string `json:"committedAt"`
	// Freshness is this interface's own judgement of its fetch loop, which is
	// the only freshness a human here can act on.
	Freshness freshnessPayload `json:"freshness"`
	// Stale is Freshness.State != "current", kept for one release so that a
	// reader written against the old shape still parses. Nothing in this
	// build reads it.
	Stale             bool         `json:"stale"`
	StaleAfterSeconds int          `json:"staleAfterSeconds"`
	SyncMode          string       `json:"syncMode"`
	StateRoot         string       `json:"stateRoot"`
	Message           string       `json:"message"`
	Problems          []string     `json:"problems"`
	Fetch             fetchPayload `json:"fetch"`
}

// freshnessPayload is the one judgement three readers share: the board's sync
// chip, the board's ledger statement, and the Overview's health pill. Since
// is the instant the state is measured from — the last fetch that landed, or
// the failure itself — and Detail is why, in the engine's own words.
type freshnessPayload struct {
	State  snapshot.Freshness `json:"state"`
	Since  string             `json:"since"`
	Detail string             `json:"detail"`
}

type fetchPayload struct {
	Outcome    string `json:"outcome"`
	StartedAt  string `json:"startedAt"`
	FinishedAt string `json:"finishedAt"`
	Tip        string `json:"tip"`
	Detail     string `json:"detail"`
	Message    string `json:"message"`
	Failures   int    `json:"failures"`
	Cadence    string `json:"cadence"`
	NextAt     string `json:"nextAt"`
	// SucceededAt and SucceededTip are the last look that landed, which a
	// running tick and a failure both leave standing.
	SucceededAt  string `json:"succeededAt"`
	SucceededTip string `json:"succeededTip"`
}

type admissionPayload struct {
	Answered bool   `json:"answered"`
	Message  string `json:"message"`
}

// workingTreePayload counts the checkout's goal files where no accepted tip
// can be read. A null count is "not known", never none.
type workingTreePayload struct {
	LiveFiles     *int `json:"liveFiles"`
	ArchivedFiles *int `json:"archivedFiles"`
}

// fetchQuery is what the board's Refresh adds to the read. Refresh is a human
// asking "is this current now", and an answer composed from a loop that last
// looked half a cadence ago is an answer about a moment that has passed. So
// the ask runs one look first and then observes, and what the human reads is
// the truth of the instant they pressed it. Every other read of this resource
// — the mount, an act's answer, the Overview's composition — observes and
// starts nothing, exactly as before.
const fetchQuery = "fetch"

// backlog answers what the accepted ledger says. Every ledger state answers
// 200, including the ones that carry no rows: the workspace answered, and the
// answer is why the ledger cannot be read. A 500 is for an engine that cannot
// answer at all.
func (h *handler) backlog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Observe == nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, "this engine was built without a ledger reader")
		return
	}
	// The look is bounded by the loop's own budget, so a hung transport ends
	// this request rather than holding it open: the caller is the same
	// process's loop machinery, and it kills what it started.
	if h.info.Fetch != nil && r.URL.Query().Has(fetchQuery) {
		h.info.Fetch()
	}
	_ = json.NewEncoder(w).Encode(h.backlogPayload())
}

// backlogPayload is the whole resource: what the accepted ledger says, what
// this server may do to it, and the budget law an approval prefills from. The
// two act routes answer with it too, so a board that moves a card is moving
// it because the ledger moved.
func (h *handler) backlogPayload() backlogPayload {
	observed := h.info.Observe()
	payload := backlogOf(observed)
	h.joinHolders(observed, &payload)
	payload.Authority = authorityPayload{
		Proven: h.info.Authority.Proven,
		Human:  h.info.Authority.Human,
		Reason: h.info.Authority.Reason,
	}
	if h.info.BudgetDefaults != nil {
		if defaults, err := h.info.BudgetDefaults(); err == nil && defaults != nil {
			payload.BudgetDefaults = defaults
		}
	}
	return payload
}

// joinHolders flags the claimed rows whose holder has gone silent, from the
// same observation the rows were projected from.
//
// It is best effort by design: a presence copy this seat cannot read costs
// the board a flag and never a row. The board's whole job is to say what the
// ledger says, and the ledger said it whether or not anybody has heard from
// the machine holding it.
func (h *handler) joinHolders(observed snapshot.Observation, payload *backlogPayload) {
	if h.info.Fleet == nil || len(payload.Rows) == 0 {
		return
	}
	page, err := h.info.Fleet(observed, backlog.Board{Rows: plainRows(payload.Rows)}, h.now())
	if err != nil {
		return
	}
	for index := range payload.Rows {
		payload.Rows[index].Holder = holderOf(page, payload.Rows[index].Row)
	}
}

func writeError(w io.Writer, reason string) {
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
	}{Error: reason})
}

func backlogOf(observation snapshot.Observation) backlogPayload {
	payload := backlogPayload{
		SchemaVersion: backlogSchemaVersion,
		ObservedAt:    stamp(observation.ObservedAt),
		Ledger: ledgerPayload{
			State:             string(observation.State),
			Tip:               observation.Tip,
			CommittedAt:       stamp(observation.CommittedAt),
			StaleAfterSeconds: int(goal.StaleThreshold / time.Second),
			SyncMode:          observation.SyncMode,
			StateRoot:         observation.StateRoot,
			Message:           observation.Message,
			Problems:          problemLines(observation.Problems),
			Fetch:             fetchOf(observation.Fetch),
		},
		WorkingTree:    workingTreePayload{LiveFiles: observation.LiveFiles, ArchivedFiles: observation.ArchivedFiles},
		BudgetDefaults: map[string]goalbudget.Budget{},
		Counts:         map[backlog.Lane]int{},
		Draft:          backlog.DraftGap{Statement: backlog.DraftStatement},
		Rows:           []boardRow{},
		Closed:         []boardRow{},
	}
	payload.Ledger.Freshness = freshnessOf(observation)
	payload.Ledger.Stale = payload.Ledger.Freshness.State != snapshot.FreshnessCurrent
	if observation.State != snapshot.StateRead {
		return payload
	}
	payload.Admission = admissionPayload{
		Answered: observation.Admission.Answered,
		Message:  observation.Admission.Message,
	}
	board := backlog.Project(observation.Tree, observation.Horizon, observation.Admission)
	payload.Counts, payload.Draft = board.Counts, board.Draft
	payload.Rows, payload.Closed = wrapRows(board.Rows), wrapRows(board.Closed)
	return payload
}

func fetchOf(state snapshot.FetchState) fetchPayload {
	return fetchPayload{
		Outcome:      string(state.Outcome),
		StartedAt:    stamp(state.StartedAt),
		FinishedAt:   stamp(state.FinishedAt),
		Tip:          state.Tip,
		Detail:       state.Detail,
		Message:      state.Message,
		Failures:     state.Failures,
		Cadence:      state.Cadence,
		NextAt:       stamp(state.NextAt),
		SucceededAt:  stamp(state.SucceededAt),
		SucceededTip: state.SucceededTip,
	}
}

// freshnessOf judges this interface's freshness by its own fetch loop, and by
// nothing else.
//
// The threshold is the engine's, applied to the fetch rather than to the
// commit. The two are different questions: a commit's age says how busy the
// project has been, and a fetch's age says whether this clone has looked
// lately. Only the second is something the human in front of this page can
// change, so only the second is ever reported as a warning — a quiet ledger
// is current, and Refresh leaves it that way.
//
// The failure is answered first because it is the loudest fact the loop has:
// a clone that cannot reach the canonical branch is not merely behind, and
// saying "behind" for it would hide the cause a human has to fix.
func freshnessOf(observation snapshot.Observation) freshnessPayload {
	loop := observation.Fetch
	if loop.Outcome == snapshot.OutcomeFailed {
		return freshnessPayload{
			State: snapshot.FreshnessFailed, Since: stamp(loop.FinishedAt),
			Detail: failureDetail(loop),
		}
	}
	landed := stamp(loop.SucceededAt)
	switch {
	case loop.SucceededAt.IsZero():
		return freshnessPayload{
			State:  snapshot.FreshnessBehind,
			Detail: "no fetch of the canonical branch has completed on this clone yet",
		}
	case observation.ObservedAt.Sub(loop.SucceededAt) > goal.StaleThreshold:
		return freshnessPayload{
			State: snapshot.FreshnessBehind, Since: landed,
			Detail: "the last fetch of the canonical branch landed more than " +
				humanDuration(goal.StaleThreshold) + " ago",
		}
	case loop.SucceededTip != "" && observation.Tip != "" && observation.Tip != loop.SucceededTip:
		return freshnessPayload{
			State: snapshot.FreshnessBehind, Since: landed,
			Detail: "the last fetch found the canonical branch at " + shortTip(loop.SucceededTip) +
				" and this clone has accepted " + shortTip(observation.Tip),
		}
	}
	return freshnessPayload{State: snapshot.FreshnessCurrent, Since: landed, Detail: loop.Detail}
}

// failureDetail is the loop's own message with the count of failures behind
// it, because one refused fetch and an hour of refused fetches are different
// situations and the message alone cannot tell them apart.
func failureDetail(loop snapshot.FetchState) string {
	message := loop.Message
	if message == "" {
		message = "the fetch of the canonical branch failed"
	}
	if loop.Failures <= 1 {
		return message
	}
	return message + " (" + strconv.Itoa(loop.Failures) + " fetches in a row have failed)"
}

// shortTip is the object name a human compares by eye, as every other reader
// of a tip in this interface writes it.
func shortTip(tip string) string {
	if len(tip) > 7 {
		return tip[:7]
	}
	return tip
}

// stamp writes an instant a reader can parse, and an empty string for an
// instant nothing recorded.
func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

func problemLines(problems []goal.Problem) []string {
	lines := make([]string, 0, len(problems))
	for _, problem := range problems {
		lines = append(lines, string(problem))
	}
	return lines
}
