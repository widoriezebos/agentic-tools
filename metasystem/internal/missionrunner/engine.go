package missionrunner

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/events"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The runner engine: one Engine drives one mission's lifecycle — launching
// the detached run loop, cycling host turns, and answering status and ask
// queries. The decision surfaces earlier files own (adjudication, conclusion
// proposals, job selection) stay pure; the Engine is the single writer that
// applies them to disk.

// RunnerError is a refusal carrying the exit code the runner maps it to.
type RunnerError struct {
	Code    int
	Message string
}

func (e *RunnerError) Error() string { return e.Message }

// failf builds a refusal with an explicit exit code.
func failf(code int, format string, args ...any) *RunnerError {
	return &RunnerError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// exitFor maps any error to the runner's exit code: a refusal keeps its own
// code, anything else is the generic failure 3.
func exitFor(err error) int {
	if refusal, ok := err.(*RunnerError); ok {
		return refusal.Code
	}
	return 3
}

// Engine drives one mission's runner lifecycle under one checkout root. Its
// event stream is the flight-recorder witness: emitting never fails a caller.
type Engine struct {
	Root    string
	Mission string
	emitter events.Emitter
	// anchorFn overrides how a state advance is anchored. Production always
	// anchors through the binary (anchorState); tests inject it because the
	// anchor is an external git effect a unit test cannot shell out for.
	anchorFn func(statePath, ledgerPath, identityName string) error
	// custodianFn overrides the custodian prover for tests. Production binds
	// identity.Custodian, the kernel custodian discipline the standing
	// reaper judges by, so the runner's drain reap can never disagree with
	// it about one record.
	custodianFn func(pid, start int64, tag string) identity.Liveness
}

// anchor writes the state's anchor commit through the configured anchorer.
func (e *Engine) anchor(statePath, ledgerPath, identityName string) error {
	if e.anchorFn != nil {
		return e.anchorFn(statePath, ledgerPath, identityName)
	}
	return e.anchorState(statePath, ledgerPath, identityName)
}

// NewEngine builds the engine for one mission. The emitter's process identity
// is completed by the run loop once it can prove its own start time; every
// other entry point emits with pidStartedAt zero, matching a process that has
// not established its recorded identity yet.
func NewEngine(root, mission string) *Engine {
	return &Engine{
		Root:    root,
		Mission: mission,
		emitter: events.Emitter{Component: "runner", Pid: int64(os.Getpid())},
	}
}

// emit records one flight-recorder event, dropping empty fields.
func (e *Engine) emit(event, summary string, fields map[string]string) {
	kept := map[string]string{}
	for key, value := range fields {
		if value != "" {
			kept[key] = value
		}
	}
	e.emitter.Emit(e.Root, event, summary, kept)
}

// clipSummary bounds a free-text summary before it enters the event stream.
func clipSummary(text string) string {
	if len(text) > 200 {
		return text[:200]
	}
	return text
}

// nowISO is the timestamp format every runner artifact carries.
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// randomHex returns n random bytes as lowercase hex, for turn ids, instance
// tags, and start-signal names.
func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		// The system CSPRNG failing is not something the runner can work
		// around; every identity it mints would collide.
		panic(err)
	}
	return hex.EncodeToString(buf)
}

// missionDir is the mission's artifact directory.
func (e *Engine) missionDir() string {
	return missionDirPath(e.Root, e.Mission)
}

// contractPath is the authored mission contract in plans/.
func (e *Engine) contractPath() string {
	return filepath.Join(e.Root, "plans", "mission-"+e.Mission+".contract.md")
}

// approvedContractPath is the pinned snapshot the mission runs against.
func (e *Engine) approvedContractPath() string {
	return filepath.Join(e.missionDir(), "mission-"+e.Mission+".contract.md")
}

// runnerPaths are the runner's record, heartbeat, and log files.
func (e *Engine) runnerPaths() (record, heartbeat, log string) {
	dir := filepath.Join(e.Root, "artifacts", "agents", "missions", "runners")
	return filepath.Join(dir, e.Mission+".json"),
		filepath.Join(dir, e.Mission+".heartbeat"),
		filepath.Join(dir, e.Mission+".log")
}

// readDocLabeled reads a JSON object, labeling the failure with the artifact's
// name and the exit code its unreadability maps to.
func readDocLabeled(path, label string, code int) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, failf(code, "%s is unreadable: %s: %v", label, path, err)
	}
	doc, err := decodeJSONDoc(data)
	if errors.Is(err, errNotJSONObject) {
		return nil, failf(code, "%s must be a JSON object: %s", label, path)
	}
	if err != nil {
		return nil, failf(code, "%s is unreadable: %s: %v", label, path, err)
	}
	return doc, nil
}

// atomicWriteBytes writes a file atomically and durably: temp file in the
// target directory, fsync, rename, directory fsync.
func atomicWriteBytes(path string, data []byte) error {
	// Through the durable-write owner (go-production-grade B5); the
	// empty anchor preserves this writer's previous behavior exactly
	// until its caller is converted to the two-outcome contract.
	_, err := atomicfile.WriteText(path, string(data), "")
	return err
}

// atomicWriteJSON writes indented, key-sorted JSON with a trailing newline.
func atomicWriteJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteBytes(path, append(data, '\n'))
}

// atomicWriteText writes a small text artifact atomically.
func atomicWriteText(path, text string) error {
	return atomicWriteBytes(path, []byte(text))
}

// jsonInt reads a JSON number as an integer, whichever decoder produced it.
func jsonInt(value any) (int64, bool) {
	switch number := value.(type) {
	case json.Number:
		parsed, err := number.Int64()
		return parsed, err == nil
	case float64:
		if number != float64(int64(number)) {
			return 0, false
		}
		return int64(number), true
	case int:
		return int64(number), true
	case int64:
		return number, true
	}
	return 0, false
}

// valueString renders a JSON scalar the way the runner's records expect:
// number literals as written, strings as themselves.
func valueString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// intFromString parses a contract integer value.
func intFromString(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("not an integer: %q", raw)
	}
	return value, nil
}
