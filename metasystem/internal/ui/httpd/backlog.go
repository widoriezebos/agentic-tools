package httpd

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// backlogPath is the backlog resource, matched exactly: what lies beneath it
// belongs to no resource, so it is a 404 like any other unserved path under a
// reserved prefix.
const backlogPath = "/api/backlog"

// backlogSchemaVersion is the shape of the resource a reader parses.
const backlogSchemaVersion = 1

type backlogPayload struct {
	SchemaVersion int                  `json:"schemaVersion"`
	ObservedAt    string               `json:"observedAt"`
	Ledger        ledgerPayload        `json:"ledger"`
	Admission     admissionPayload     `json:"admission"`
	WorkingTree   workingTreePayload   `json:"workingTree"`
	Counts        map[backlog.Lane]int `json:"counts"`
	Draft         backlog.DraftGap     `json:"draft"`
	Rows          []backlog.Row        `json:"rows"`
	Closed        []backlog.Row        `json:"closed"`
}

type ledgerPayload struct {
	State string `json:"state"`
	Tip   string `json:"tip"`
	// CommittedAt is the accepted commit's committer time, which is the age
	// of the tree; nothing records when this clone accepted it.
	CommittedAt       string       `json:"committedAt"`
	Stale             bool         `json:"stale"`
	StaleAfterSeconds int          `json:"staleAfterSeconds"`
	SyncMode          string       `json:"syncMode"`
	StateRoot         string       `json:"stateRoot"`
	Message           string       `json:"message"`
	Problems          []string     `json:"problems"`
	Fetch             fetchPayload `json:"fetch"`
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

// backlog answers what the accepted ledger says. Every ledger state answers
// 200, including the ones that carry no rows: the workspace answered, and the
// answer is why the ledger cannot be read. A 500 is for an engine that cannot
// answer at all.
func (h *handler) backlog(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Observe == nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeError(w, "this engine was built without a ledger reader")
		return
	}
	_ = json.NewEncoder(w).Encode(backlogOf(h.info.Observe()))
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
		WorkingTree: workingTreePayload{LiveFiles: observation.LiveFiles, ArchivedFiles: observation.ArchivedFiles},
		Counts:      map[backlog.Lane]int{},
		Draft:       backlog.DraftGap{Statement: backlog.DraftStatement},
		Rows:        []backlog.Row{},
		Closed:      []backlog.Row{},
	}
	// An age needs a commit time. Without one the tree's age is unknown,
	// which is not the same as fresh, so nothing is claimed about it.
	if !observation.CommittedAt.IsZero() {
		payload.Ledger.Stale = observation.ObservedAt.Sub(observation.CommittedAt) > goal.StaleThreshold
	}
	if observation.State != snapshot.StateRead {
		return payload
	}
	payload.Admission = admissionPayload{
		Answered: observation.Admission.Answered,
		Message:  observation.Admission.Message,
	}
	board := backlog.Project(observation.Tree, observation.Horizon, observation.Admission)
	payload.Counts, payload.Draft = board.Counts, board.Draft
	payload.Rows, payload.Closed = board.Rows, board.Closed
	return payload
}

func fetchOf(state snapshot.FetchState) fetchPayload {
	return fetchPayload{
		Outcome:    string(state.Outcome),
		StartedAt:  stamp(state.StartedAt),
		FinishedAt: stamp(state.FinishedAt),
		Tip:        state.Tip,
		Detail:     state.Detail,
		Message:    state.Message,
		Failures:   state.Failures,
		Cadence:    state.Cadence,
		NextAt:     stamp(state.NextAt),
	}
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
