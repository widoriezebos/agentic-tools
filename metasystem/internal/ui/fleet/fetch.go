package fleet

// The interface's own presence fetch.
//
// It is one owner, started and stopped under the server's ownership gate,
// beside the snapshot loop and never through it: that loop's Fetch returns
// one error, and a failure there blanks the ledger tip and backs the whole
// loop off toward five minutes. A presence fetch that failed must cost the
// fleet page its freshness and the ledger nothing.
//
// Its rules, all of them:
//
//   - one attempt in flight;
//   - at least a minute between attempt starts, counting failures and
//     reconnects, because a browser that reconnects every few seconds must
//     not become a fetch every few seconds;
//   - an attempt only while at least one browser holds the notifications
//     stream open, which is this server's one explicit connection signal —
//     the snapshot loop's own "connected" is a private observe-within-thirty
//     seconds state and is not read here;
//   - no attempt after the last disconnect.
//
// It keeps last attempt, last success and last failure apart. A failure
// preserves the last success and the refs that success brought; a success
// that brought nothing is a success with an empty copy, which is a different
// thing from never having fetched. Nothing survives a restart: the page says
// so until the first attempt.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// MinInterval is the floor between two attempt starts.
const MinInterval = time.Minute

// PollInterval is how often the owner wakes to ask whether an attempt is due.
// It is not the fetch cadence: MinInterval is.
const PollInterval = 5 * time.Second

// MetadataSchema is the shape of the file the Partner's tool reads.
const MetadataSchema = 1

/* ------------------------------------------------------------- the watch -- */

// Watch is the bridge between the browsers and the fetch owner: it counts the
// notification streams that are open, which is the owner's connection signal,
// and carries the one `fleet` event back to every one of them after an
// attempt.
//
// It is one object because the two are one fact per stream: a page that is
// listening is a page that is connected, and a page that has gone is neither.
type Watch struct {
	mu       sync.Mutex
	next     int
	watchers map[int]chan struct{}
}

// NewWatch builds an empty watch.
func NewWatch() *Watch { return &Watch{watchers: map[int]chan struct{}{}} }

// Join registers one open browser stream. The channel carries one signal per
// presence attempt, and the returned function ends the registration — which
// is also what stops this stream counting as a connection.
func (w *Watch) Join() (<-chan struct{}, func()) {
	signals := make(chan struct{}, 1)
	if w == nil {
		return signals, func() {}
	}
	w.mu.Lock()
	w.next++
	id := w.next
	w.watchers[id] = signals
	w.mu.Unlock()
	return signals, func() {
		w.mu.Lock()
		delete(w.watchers, id)
		w.mu.Unlock()
	}
}

// Connected reports whether any browser holds the stream open.
//
// It answers about the instant it was asked and nothing after it, which is
// why nothing decides whether to fetch from it: see Admit.
func (w *Watch) Connected() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.watchers) > 0
}

// Admit runs decide while no stream can join or leave, hands it the
// connection as it stands, and answers what it decided.
//
// The connection signal and the decision that reads it are one act. Asking
// Connected and then taking the attempt slot is two, and the last browser can
// leave between them — which is exactly the case the rule exists to prevent:
// no attempt after the last disconnect. So the removal and the admission are
// ordered under this one lock.
//
// decide must not fetch. It decides, and the attempt runs after this returns,
// because holding every stream's registration for the length of a bounded git
// call would block a browser arriving or leaving for up to a minute.
func (w *Watch) Admit(decide func(connected bool) bool) bool {
	if w == nil {
		return decide(false)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return decide(len(w.watchers) > 0)
}

// Announce tells every open stream that a presence attempt finished, whatever
// it found. The signal carries nothing: the page re-reads /api/fleet, which is
// the one place the answer is composed.
//
// A watcher that has not drained its last signal is left alone rather than
// waited for: one pending "read again" and two pending "read again" ask for
// the same single read.
func (w *Watch) Announce() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, signals := range w.watchers {
		select {
		case signals <- struct{}{}:
		default:
		}
	}
}

/* ------------------------------------------------------------ the owner -- */

// Metadata is what the fetch owner writes down after every attempt, for the
// Partner's tool, which runs in another process and owns no fetcher of its
// own.
//
// Run is what makes it readable at all. The file outlives the server that
// wrote it: nothing the fetch owner knows survives a restart, and the refs a
// previous server brought into the interface's namespace are still there. A
// tool that read this file after a restart would cite a success no running
// server has made, and would read the interface's own namespace while the
// page — which knows better — had fallen back to the tick's copy. So the file
// names the run that wrote it, and a reader that cannot show it belongs to
// the run it is talking to treats it as another server's.
type Metadata struct {
	Schema      int    `json:"schema"`
	Run         string `json:"run,omitempty"`
	Namespace   string `json:"namespace"`
	AttemptedAt string `json:"attemptedAt,omitempty"`
	SucceededAt string `json:"succeededAt,omitempty"`
	FailedAt    string `json:"failedAt,omitempty"`
	Problem     string `json:"problem,omitempty"`
}

// OfRun reports whether this metadata was written by the run named here. An
// unnamed run on either side is not a match: a reader that cannot say which
// server it is talking to has no business citing that server's fetches.
func (m Metadata) OfRun(run string) bool {
	return run != "" && m.Run == run
}

// MetadataPath is where the fetch owner writes that file.
func MetadataPath(checkout string) string {
	return filepath.Join(checkout, "artifacts", "agents", "ui", "presence-fetch.json")
}

// LoadMetadata reads it. An absent file is a server that has not fetched —
// or is not running at all — which is not an error.
func LoadMetadata(checkout string) (Metadata, bool, error) {
	data, err := os.ReadFile(MetadataPath(checkout))
	if err != nil {
		if os.IsNotExist(err) {
			return Metadata{Schema: MetadataSchema, Namespace: seat.UINamespace}, false, nil
		}
		return Metadata{}, false, err
	}
	var read Metadata
	if err := json.Unmarshal(data, &read); err != nil {
		return Metadata{}, true, err
	}
	return read, true, nil
}

// SaveMetadata writes it durably beside the steward's own agent files.
func SaveMetadata(checkout string, state Metadata) error {
	state.Schema = MetadataSchema
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(MetadataPath(checkout), string(append(data, '\n')), checkout)
	return err
}

// Owner is the interface's one presence fetcher.
type Owner struct {
	// Attempt is one bounded presence fetch into the interface's namespace.
	// It is the seat package's transport and never the snapshot loop's.
	Attempt func() error
	// Admit is the connection signal and the attempt slot, taken together:
	// it runs the decision while no stream can join or leave, so an attempt
	// can never start after the last disconnect. Watch.Admit is what the
	// server passes.
	Admit func(decide func(connected bool) bool) bool
	// Announce is called after every attempt, success or failure or nothing
	// found, so a mounted page learns there is something to re-read.
	Announce func()
	// Record writes the metadata file. A record that fails does not fail the
	// fetch — the copy is in the namespace either way — but it is not
	// silence: the tool is reading a file that has stopped moving, and the
	// page says so rather than leaving a reader to wonder.
	Record func(Metadata) error
	// RunID is this server run's own identifier, written into every metadata
	// file so a reader can tell this run's fetches from a previous one's. It
	// is not the Run method below, which is the loop.
	RunID string
	// Now is this owner's clock.
	Now func() time.Time
	// Interval is the floor between attempt starts; zero takes MinInterval.
	Interval time.Duration

	mu        sync.Mutex
	inFlight  bool
	started   time.Time
	attempted time.Time
	succeeded time.Time
	failed    time.Time
	problem   string
	// recorded is the last metadata write's own trouble, kept apart from the
	// fetch's: one says the copy could not be brought in, the other that the
	// Partner's tool cannot be told it was.
	recorded string
}

// State is what the owner knows, as the page's copy block.
//
// Source is the caller's to decide, because it depends on which namespace the
// request actually read; this says only what this owner did.
func (o *Owner) State() Copy {
	o.mu.Lock()
	defer o.mu.Unlock()
	return Copy{
		AttemptedAt:     instant(o.attempted),
		SucceededAt:     instant(o.succeeded),
		FailedAt:        instant(o.failed),
		Problem:         o.problem,
		MetadataProblem: o.recorded,
	}
}

// Succeeded reports whether this owner has ever brought a copy in, which is
// what decides whether the page reads the interface's namespace or falls back
// to the tick's.
func (o *Owner) Succeeded() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return !o.succeeded.IsZero()
}

// Consider makes one attempt if this instant allows one, and reports whether
// it did. It is the whole of the owner's policy, so a test drives the rules
// with an injected clock and a fake connection signal and never a wall.
func (o *Owner) Consider() bool {
	started := o.now()
	if !o.begin(started) {
		return false
	}
	err := o.Attempt()
	// The clock is read again here. A bounded fetch may take the whole of its
	// budget, and stamping the success with the instant the attempt STARTED
	// would date the copy a minute earlier than it is — which is exactly the
	// number the page's "presence fetched 40 s ago" is measured from.
	o.end(started, o.now(), err)
	if o.Record != nil {
		state := o.State()
		o.recordWrote(o.Record(Metadata{
			Run: o.RunID, Namespace: seat.UINamespace, AttemptedAt: state.AttemptedAt,
			SucceededAt: state.SucceededAt, FailedAt: state.FailedAt, Problem: state.Problem,
		}))
	}
	if o.Announce != nil {
		o.Announce()
	}
	return true
}

// begin takes the one attempt slot, under the three rules, and reports
// whether this caller got it.
//
// The connected rule is decided inside Admit rather than before it, so that a
// browser leaving cannot slip between "somebody is watching" and "this
// attempt is now in flight".
func (o *Owner) begin(now time.Time) bool {
	if o.Attempt == nil || o.Admit == nil {
		return false
	}
	return o.Admit(func(connected bool) bool {
		if !connected {
			return false
		}
		o.mu.Lock()
		defer o.mu.Unlock()
		if o.inFlight {
			return false
		}
		if !o.started.IsZero() && now.Sub(o.started) < o.interval() {
			return false
		}
		o.inFlight = true
		o.started = now
		return true
	})
}

// end records what the attempt found. A failure leaves the last success and
// the refs it brought exactly where they were.
//
// started is when the attempt began and finished is when it came back: the
// attempt is dated from the one, and what it found from the other.
func (o *Owner) end(started, finished time.Time, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.inFlight = false
	o.attempted = started
	if err != nil {
		o.failed = finished
		o.problem = err.Error()
		return
	}
	o.succeeded = finished
	o.problem = ""
}

// recordWrote keeps the last metadata write's outcome, so a tool reading a
// file that has stopped moving is something the page can say.
func (o *Owner) recordWrote(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err == nil {
		o.recorded = ""
		return
	}
	o.recorded = "the Partner's presence metadata could not be written: " + err.Error()
}

func (o *Owner) interval() time.Duration {
	if o.Interval <= 0 {
		return MinInterval
	}
	return o.Interval
}

func (o *Owner) now() time.Time {
	if o.Now == nil {
		return time.Now().UTC()
	}
	return o.Now().UTC()
}

// Run drives the owner until the context ends, considering an attempt on
// every tick. The tick is the caller's, so the server passes a real ticker
// and a test passes a channel it controls.
func (o *Owner) Run(ctx context.Context, tick <-chan time.Time) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, open := <-tick:
			if !open {
				return
			}
			o.Consider()
		}
	}
}

func instant(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return seat.FormatTime(at)
}
