// Package outage is the shared record of a model-provider outage: one
// small mark every layer can write when a provider call comes back
// overloaded (529/overloaded/5xx) and every layer can read to stop
// blaming local machinery for the provider's weather. The mark is a
// HEALTH HINT, not a ledger: writers race last-write-wins, a torn or
// unreadable mark reads as no outage, and consumers must stay correct
// without it. A standing mark pauses the steward's patience clocks and
// keeps provider failures off the mission runner's host-failure
// breaker; it never authorizes, blocks, or excuses anything else.
//
// The mark must be FED to keep standing: each new overload failure
// refreshes it, and a mark older than Horizon lapses. Without the
// horizon a mark written once and never cleared — every provider
// consumer gone quiet, so no success ever clears it — would blind the
// steward to a genuine stall forever.
package outage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Horizon is how long a standing outage survives without a new failure
// feeding it. Provider retries and steward-tick revival probes arrive
// well inside this window during a real outage.
const Horizon = 30 * time.Minute

// evidenceClip bounds the stored evidence line.
const evidenceClip = 200

// Mark is the outage record: how many provider failures in a row, what
// the last one looked like, who saw it, and when the outage began.
type Mark struct {
	ConsecutiveFailures int    `json:"consecutiveFailures"`
	LastClass           string `json:"lastClass"`
	LastDetail          string `json:"lastDetail"`
	Source              string `json:"source"`
	Since               string `json:"since"`
	LastAt              string `json:"lastAt"`
}

// Path is the mark's one location under the repository root.
func Path(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "outage.json")
}

// Read returns the raw mark. A missing, torn, or unreadable file is no
// outage — the hint fails toward normal operation, never toward a
// paused clock nobody asked for.
func Read(repoRoot string) (Mark, bool) {
	data, err := os.ReadFile(Path(repoRoot))
	if err != nil {
		return Mark{}, false
	}
	var m Mark
	if json.Unmarshal(data, &m) != nil || m.ConsecutiveFailures < 1 {
		return Mark{}, false
	}
	return m, true
}

// StandingAt is Read with the horizon applied: a mark whose last
// feeding is older than Horizon has lapsed. A LastAt that does not
// parse lapses too — an unreadable age must not stand forever — and
// so does one more than Horizon in the FUTURE: a clock correction or
// a corrupt stamp must not pause the clocks beyond the same bound the
// horizon promises.
func StandingAt(repoRoot string, now time.Time) (Mark, bool) {
	m, ok := Read(repoRoot)
	if !ok {
		return Mark{}, false
	}
	last, err := time.Parse(time.RFC3339, m.LastAt)
	if err != nil {
		return Mark{}, false
	}
	if age := now.Sub(last); age > Horizon || age < -Horizon {
		return Mark{}, false
	}
	return m, true
}

// markLock serializes Record and Clear across processes, so a torn
// read-modify-write can never resurrect a cleared outage or regress a
// newer feeding under an older one. What the lock cannot fix is the
// observation race itself: a failure OBSERVED before a success but
// recorded after it re-marks the outage — the next success clears it
// again, and the horizon bounds the damage either way.
type markLock struct{ f *os.File }

// The acquire is BOUNDED: a hint must never wedge its caller. Mission
// runners call Clear while holding their mission lease — a process
// stuck holding this lock may cost the hint an update, never the
// runner its turn.
func acquireMarkLock(repoRoot string) (*markLock, error) {
	path := filepath.Join(repoRoot, "artifacts", "agents", "outage.flock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 10; attempt++ {
		err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return &markLock{f: f}, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	f.Close()
	return nil, fmt.Errorf("outage mark lock is held; the hint update is skipped: %w", err)
}

func (l *markLock) release() {
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	_ = l.f.Close()
}

// Record folds one provider-overload failure into the mark and returns
// the updated mark. A lapsed or absent mark starts a new outage. The
// stamp never regresses: a slower writer with an older clock cannot
// age a mark a faster one just fed (RFC3339 UTC compares lexically).
func Record(repoRoot, class, detail, source string, now time.Time) (Mark, error) {
	lock, err := acquireMarkLock(repoRoot)
	if err != nil {
		return Mark{}, err
	}
	defer lock.release()
	m, standing := StandingAt(repoRoot, now)
	if !standing {
		m = Mark{Since: now.UTC().Format(time.RFC3339)}
	}
	m.ConsecutiveFailures++
	m.LastClass = class
	m.LastDetail = clip(detail)
	m.Source = source
	if stamp := now.UTC().Format(time.RFC3339); !standing || m.LastAt < stamp {
		m.LastAt = stamp
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return Mark{}, err
	}
	if err := atomicfile.WriteVolatile(Path(repoRoot), string(data)+"\n"); err != nil {
		return Mark{}, err
	}
	return m, nil
}

// Clear removes the mark: any provider success ends the outage. An
// already-absent mark is success.
func Clear(repoRoot string) error {
	lock, err := acquireMarkLock(repoRoot)
	if err != nil {
		return err
	}
	defer lock.release()
	err = os.Remove(Path(repoRoot))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// The line rules for provider-overload evidence (Wido's ruling:
// 529/overloaded/5xx). overloadCodeRe is any HTTP 5xx status ADJACENT
// to its framing vocabulary, both sides bounded — "status 503",
// "HTTP 502", "API Error: 529", "code=500" — so a duration like
// "error after 500ms", an offset like "error at line 15290", and a
// word ending in the vocabulary like "failed to decode 500 records"
// can never invent an outage. The word "overloaded" counts only with
// provider framing beside it — the provider's own error token, an API
// error phrase, or a 5xx match on the same line — because logs also
// carry local prose ("the scheduler is overloaded") that convicts no
// provider.
var (
	overloadWordRe = regexp.MustCompile(`(?i)(?:^|[^a-z])overloaded(?:[^a-z]|$)`)
	overloadCodeRe = regexp.MustCompile(`(?i)(?:^|[^a-z])(?:status|http|code|error)[^a-z0-9]{1,4}(5[0-9][0-9])(?:[^0-9a-z]|$)`)
)

// classifyLine names a line of provider-error evidence; empty means
// not overload-shaped.
func classifyLine(line string) string {
	l := strings.ToLower(line)
	code := overloadCodeRe.FindStringSubmatch(line)
	if overloadWordRe.MatchString(line) {
		if strings.Contains(l, "overloaded_error") || strings.Contains(l, "api error") || code != nil {
			return "overloaded"
		}
		return ""
	}
	if code != nil {
		return "http-" + code[1]
	}
	return ""
}

// logScanCap bounds how much of a log tail the classifier reads.
const logScanCap = 64 * 1024

// ClassifyLogs scans raw error-log tails for overload evidence and
// returns the class, the clipped matching line, and whether anything
// hit. ONLY logs belong here: a log carries diagnostics, never model
// output, so a match means the provider spoke. Model-visible files
// (results, replies) go through ClassifyProviderResult's structured
// gate instead — a model merely DISCUSSING a 529 must not mark one.
func ClassifyLogs(paths ...string) (class, evidence string, ok bool) {
	for _, path := range paths {
		if path == "" {
			continue
		}
		text, err := readTail(path, logScanCap)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(text, "\n") {
			if c := classifyLine(line); c != "" {
				return c, clip(strings.TrimSpace(line)), true
			}
		}
	}
	return "", "", false
}

// ClassifyProviderResult consults a structured provider result file: it
// classifies only when the document declares itself an error
// (is_error), and then only the document's own strings — the gate that
// keeps model output out of the outage record.
func ClassifyProviderResult(path string) (class, evidence string, ok bool) {
	if path == "" {
		return "", "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", false
	}
	var doc map[string]any
	if json.Unmarshal(data, &doc) != nil {
		return "", "", false
	}
	if isError, _ := doc["is_error"].(bool); !isError {
		return "", "", false
	}
	for _, field := range []string{"result", "error", "subtype", "message"} {
		if s, _ := doc[field].(string); s != "" {
			if c := classifyLine(s); c != "" {
				return c, clip(s), true
			}
		}
	}
	return "", "", false
}

// readTail reads at most cap bytes from the end of a file.
func readTail(path string, cap int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > cap {
		if _, err := f.Seek(info.Size()-cap, 0); err != nil {
			return "", err
		}
	}
	data := make([]byte, min64(info.Size(), cap))
	n, _ := f.Read(data)
	return string(data[:n]), nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func clip(s string) string {
	if len(s) <= evidenceClip {
		return s
	}
	return s[:evidenceClip]
}
