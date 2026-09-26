package launch

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"golang.org/x/sys/unix"
)

// A design request asks the design-author lane for one draft of one project
// design document. Its retained entry lives in the launch store beside the
// runs, one per canonical destination path, and holds every public attempt:
// the exact brief, the document bytes the attempt was asked to replace, the
// launch reserved for it and what became of its proposal. The per-document
// lock serializes requests, replay and publication of that document.

// DesignRequest is one caller request for a design draft.
type DesignRequest struct {
	Goal, RecordID string
	// Destination is the canonical absolute path of the project document.
	Destination string
	// WorkingDirectory is the invoking checkout the author works in.
	WorkingDirectory string
	// Brief is the caller's request; Contract is the author contract the
	// adapter prepends. Both are frozen with the attempt. Header seeds the
	// staged draft of a document that does not exist yet.
	Brief, Contract, Header []byte
	// After names the attempt this request follows; zero rejoins an
	// identical request first, then follows the newest attempt.
	After int
}

// DesignAttempt is one retained attempt.
type DesignAttempt struct {
	Attempt         int    `json:"attempt"`
	LaunchID        string `json:"launchId"`
	BriefSHA256     string `json:"briefSha256"`
	Brief           string `json:"brief"`
	Prior           string `json:"prior,omitempty"`
	ExpectedSHA256  string `json:"expectedSha256,omitempty"`
	ExpectedPresent bool   `json:"expectedPresent"`
	Draft           string `json:"draft"`
	// Outcome is published, conflict or invalid once the proposal was
	// judged against the document; empty until then.
	Outcome   string `json:"outcome,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Published string `json:"publishedSha256,omitempty"`
}

type designEntry struct {
	Goal        string          `json:"goal"`
	RecordID    string          `json:"recordId"`
	Destination string          `json:"destination"`
	Attempts    []DesignAttempt `json:"attempts"`
}

// DesignResult is the attempt a request reached and its launch record.
type DesignResult struct {
	Attempt  DesignAttempt
	Record   Record
	Rejoined bool
	Current  int
}

// ErrDesignStale is returned when After names an attempt that is not the
// newest one of the document.
var ErrDesignStale = errors.New("DESIGN_ATTEMPT_STALE")

// ErrDesignWriterRunning is returned when a new attempt is asked for while
// the newest attempt's author may still write.
var ErrDesignWriterRunning = errors.New("DESIGN_WRITER_RUNNING")

func designKey(destination string) string {
	sum := sha256.Sum256([]byte(destination))
	return fmt.Sprintf("%x", sum[:16])
}

// designRoot is the launch store's resolved root: production leaves
// Store.Root empty and the store resolves its default, so the retained
// entries never land beside whichever directory the caller runs in.
func (m *Manager) designRoot() (string, error) {
	return m.Store.root()
}

func (m *Manager) designDir(destination string) string {
	root, err := m.designRoot()
	if err != nil {
		// Unreachable after designLock or readDesignEntry checked the root.
		root = m.Store.Root
	}
	return filepath.Join(root, ".design", designKey(destination))
}

func (m *Manager) designLock(destination string) (*os.File, error) {
	if _, err := m.designRoot(); err != nil {
		return nil, err
	}
	directory := m.designDir(destination)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(directory, "lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		if lockWouldBlock(err) {
			return nil, fmt.Errorf("DESIGN_BUSY destination=%s: another caller is acting on this document; repeat the same command", destination)
		}
		return nil, err
	}
	return file, nil
}

func (m *Manager) readDesignEntry(destination string) (designEntry, bool, error) {
	if _, err := m.designRoot(); err != nil {
		return designEntry{}, false, err
	}
	data, err := os.ReadFile(filepath.Join(m.designDir(destination), "request.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return designEntry{}, false, nil
	}
	if err != nil {
		return designEntry{}, false, err
	}
	var entry designEntry
	if err := json.Unmarshal(data, &entry); err != nil || entry.Destination != destination {
		return designEntry{}, false, fmt.Errorf("DESIGN_ENTRY_CORRUPT destination=%s", destination)
	}
	return entry, true, nil
}

func (m *Manager) writeDesignEntry(entry designEntry) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(m.designDir(entry.Destination), "request.json"), string(data)+"\n", filepath.Dir(m.designDir(entry.Destination)))
	return err
}

// DesignAttempts reads a document's retained attempts without locking.
func (m *Manager) DesignAttempts(destination string) ([]DesignAttempt, error) {
	entry, _, err := m.readDesignEntry(destination)
	return entry.Attempts, err
}

// RequestDesign reaches the one attempt a request asks for. An identical
// request (same brief bytes) rejoins its attempt in any state before the
// current document is inspected; a new request freezes its brief and the
// document's current bytes, reserves the launch ID in the retained entry and
// only then starts the author. A new attempt needs the newest attempt's
// author to be terminal.
func (m *Manager) RequestDesign(request DesignRequest) (DesignResult, error) {
	if request.Destination == "" || !filepath.IsAbs(request.Destination) || request.Goal == "" || request.RecordID == "" {
		return DesignResult{}, errors.New("a design request needs its goal, record and absolute destination")
	}
	if len(request.Brief) == 0 {
		return DesignResult{}, errors.New("launch brief is empty")
	}
	lock, err := m.designLock(request.Destination)
	if err != nil {
		return DesignResult{}, err
	}
	defer releaseUnitLock(lock)
	entry, found, err := m.readDesignEntry(request.Destination)
	if err != nil {
		return DesignResult{}, err
	}
	if !found {
		entry = designEntry{Goal: request.Goal, RecordID: request.RecordID, Destination: request.Destination}
	}
	if entry.Goal != request.Goal || entry.RecordID != request.RecordID {
		return DesignResult{}, fmt.Errorf("DESIGN_DOCUMENT_CONFLICT destination=%s: the document belongs to goal %s record %s", request.Destination, entry.Goal, entry.RecordID)
	}
	briefDigest := digestHex(request.Brief)
	current := len(entry.Attempts)
	for index := len(entry.Attempts) - 1; index >= 0 && request.After == 0; index-- {
		if entry.Attempts[index].BriefSHA256 == briefDigest {
			// A reservation whose launch record was never created is the
			// interrupted start of this same attempt: start it now.
			if _, readErr := m.Store.Read(entry.Attempts[index].LaunchID); errors.Is(readErr, fs.ErrNotExist) {
				return m.startDesign(request, entry.Attempts[index])
			}
			return m.rejoinDesign(entry.Attempts[index], current)
		}
	}
	after := request.After
	if after == 0 {
		after = current
	}
	if after != current {
		return DesignResult{Current: current}, fmt.Errorf("%w destination=%s after=%d current=%d", ErrDesignStale, request.Destination, after, current)
	}
	if current > 0 {
		newest := entry.Attempts[current-1]
		if record, readErr := m.Store.Read(newest.LaunchID); readErr == nil && !record.State.Terminal() {
			if _, recovered, _ := m.RecoverStrandedStart(newest.LaunchID); !recovered {
				return DesignResult{Current: current}, fmt.Errorf("%w destination=%s attempt=%d launch=%s", ErrDesignWriterRunning, request.Destination, newest.Attempt, newest.LaunchID)
			}
		}
	}
	number := current + 1
	stage := filepath.Join(request.WorkingDirectory, "artifacts", "agents", "intent-design", fmt.Sprintf("%s-%d", request.RecordID, number))
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return DesignResult{}, err
	}
	attempt := DesignAttempt{Attempt: number, BriefSHA256: briefDigest, Draft: filepath.Join(stage, "draft.md"),
		Brief: filepath.Join(m.designDir(request.Destination), fmt.Sprintf("attempt-%d-brief.md", number))}
	if prior, readErr := os.ReadFile(request.Destination); readErr == nil {
		attempt.ExpectedPresent, attempt.ExpectedSHA256 = true, digestHex(prior)
		attempt.Prior = filepath.Join(m.designDir(request.Destination), fmt.Sprintf("attempt-%d-prior.md", number))
		if _, err := atomicfile.WriteText(attempt.Prior, string(prior), filepath.Dir(m.designDir(request.Destination))); err != nil {
			return DesignResult{}, err
		}
	} else if !errors.Is(readErr, fs.ErrNotExist) {
		return DesignResult{}, readErr
	}
	// The prompt is the only thing every runtime gives its child, so it
	// names the declared output and the frozen prior itself.
	composed := append(append([]byte{}, request.Contract...), designOutputSection(attempt, request.Header)...)
	composed = append(composed, request.Brief...)
	if _, err := atomicfile.WriteText(attempt.Brief, string(composed), filepath.Dir(m.designDir(request.Destination))); err != nil {
		return DesignResult{}, err
	}
	// Admission measures the page before the launch, so a staged copy is
	// seeded; the supervisor archives and clears it as a declared output
	// before the child starts, and the child creates its draft whole.
	seed := request.Header
	if attempt.Prior != "" {
		seed, _ = os.ReadFile(attempt.Prior)
	}
	if _, err := atomicfile.WriteText(attempt.Draft, string(seed), request.WorkingDirectory); err != nil {
		return DesignResult{}, err
	}
	if attempt.LaunchID, err = newID(m.Now()); err != nil {
		return DesignResult{}, err
	}
	entry.Attempts = append(entry.Attempts, attempt)
	if err := m.writeDesignEntry(entry); err != nil {
		return DesignResult{}, err
	}
	return m.startDesign(request, attempt)
}

// DesignOutputHeading opens the prompt section that names an author
// attempt's one declared output.
const DesignOutputHeading = "## Your output"

// designOutputSection tells the author where its one declared output is and
// what it starts from: the frozen prior version, or the reserved head.
func designOutputSection(attempt DesignAttempt, header []byte) []byte {
	text := fmt.Sprintf("\n%s\n\nWrite the complete design record to this one file, and to no other file:\n\n    %s\n\n"+
		"It does not exist when you start; create it whole.\n", DesignOutputHeading, attempt.Draft)
	if attempt.Prior != "" {
		text += fmt.Sprintf("\nThe document's current version, frozen for this attempt, is at (read it; never edit it):\n\n    %s\n", attempt.Prior)
	} else {
		text += "\nThe document does not exist yet. Begin the file with exactly this head:\n\n" + string(header)
	}
	return []byte(text + "\n## The request\n\n")
}

func (m *Manager) startDesign(request DesignRequest, attempt DesignAttempt) (DesignResult, error) {
	inputs := []string{}
	if attempt.Prior != "" {
		inputs = append(inputs, attempt.Prior)
	}
	record, err := m.Start(StartSpec{ID: attempt.LaunchID, Kind: "design", Goal: request.Goal, Tag: request.RecordID,
		WorkingDirectory: request.WorkingDirectory, Brief: attempt.Brief, Page: attempt.Draft, Inputs: inputs, Outputs: []string{attempt.Draft}})
	return DesignResult{Attempt: attempt, Record: record, Current: attempt.Attempt}, err
}

// rejoinDesign reports a retained attempt as its launch now stands; a
// reservation whose launch record was never created is started now under
// the same ID, and a stranded start is recovered as failed.
func (m *Manager) rejoinDesign(attempt DesignAttempt, current int) (DesignResult, error) {
	record, err := m.Store.Read(attempt.LaunchID)
	if err != nil {
		return DesignResult{Attempt: attempt, Rejoined: true, Current: current}, fmt.Errorf("DESIGN_LAUNCH_UNREADABLE launch=%s: %v", attempt.LaunchID, err)
	}
	if record.State == Starting {
		if recovered, ok, _ := m.RecoverStrandedStart(attempt.LaunchID); ok {
			record = recovered
		}
	}
	return DesignResult{Attempt: attempt, Record: record, Rejoined: true, Current: current}, nil
}

// RecordDesignOutcome records what became of an attempt's proposal, under
// the document lock, through judge: it reads the retained attempt and the
// draft and returns the outcome it applied to the document.
func (m *Manager) RecordDesignOutcome(destination string, attempt int, judge func(DesignAttempt) (outcome, detail, published string, err error)) (DesignAttempt, error) {
	lock, err := m.designLock(destination)
	if err != nil {
		return DesignAttempt{}, err
	}
	defer releaseUnitLock(lock)
	entry, found, err := m.readDesignEntry(destination)
	if err != nil || !found || attempt < 1 || attempt > len(entry.Attempts) {
		return DesignAttempt{}, fmt.Errorf("DESIGN_ATTEMPT_UNKNOWN destination=%s attempt=%d", destination, attempt)
	}
	retained := entry.Attempts[attempt-1]
	if retained.Outcome != "" {
		return retained, nil
	}
	if attempt != len(entry.Attempts) {
		retained.Outcome, retained.Detail = "superseded", fmt.Sprintf("attempt %d became current", len(entry.Attempts))
	} else {
		outcome, detail, published, judgeErr := judge(retained)
		if judgeErr != nil {
			return retained, judgeErr
		}
		retained.Outcome, retained.Detail, retained.Published = outcome, detail, published
	}
	entry.Attempts[attempt-1] = retained
	return retained, m.writeDesignEntry(entry)
}

// RecoverStrandedStart fails a launch whose supervisor never claimed it:
// under the record lock it must still be Starting past the start cap, with
// no supervisor, child or process group recorded and no uncertain output
// owner. A supervisor that claims first makes this refuse; once this wins,
// the supervisor's own first claim refuses the terminal record before any
// child starts. Elapsed time alone is never taken as a death proof: any
// recorded process keeps the existing proof-of-death rules.
func (m *Manager) RecoverStrandedStart(id string) (Record, bool, error) {
	recovered := false
	record, err := m.Store.Update(id, func(record *Record) error {
		if record.State != Starting || record.Supervisor != nil || record.Child != nil || record.ProcessGroup != nil || record.OutputOwnerUnproven {
			return nil
		}
		started, parseErr := time.Parse(time.RFC3339Nano, record.StartedAt)
		if parseErr != nil || m.Now().Before(started.Add(m.StartCap)) {
			return nil
		}
		record.State, record.Reason = Failed, "supervisor-start-unrecorded"
		record.FinishedAt = m.Now().UTC().Format(time.RFC3339Nano)
		recovered = true
		return nil
	})
	return record, recovered, err
}

// DesignDocument reads a document's retained entry without locking: the
// record id its first request reserved and every attempt.
func (m *Manager) DesignDocument(destination string) (recordID string, attempts []DesignAttempt, err error) {
	entry, _, err := m.readDesignEntry(destination)
	return entry.RecordID, entry.Attempts, err
}
