// Package outage is the shared record of a model-provider outage: one
// small mark every layer can write when a provider call comes back
// overloaded (529/overloaded/5xx) or limited (usage limit, rate_limit_error,
// 429) and every layer can read to stop
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
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Horizon is how long a standing outage survives without a new failure
// feeding it, unless a provider limit names a bounded reset. Provider retries
// and steward-tick revival probes arrive well inside this window.
const Horizon = 30 * time.Minute

// MaxLimitHold bounds every provider limit even when its next reset is tomorrow.
const MaxLimitHold = 5 * time.Hour

// ProviderLimit is the class of a provider's usage or rate limit: the
// Claude CLI's usage-limit line, the API's rate_limit_error, an HTTP 429.
// It is the provider's weather like an overload, so it feeds the mark.
const ProviderLimit = "provider-limit"

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
	// ResetAt is when the last failure's limit line says the limit resets;
	// a provider-limit mark stands until then, for at most MaxLimitHold.
	ResetAt string `json:"resetAt,omitempty"`
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

// ResetDefect rejects a reset that could hold work beyond the limit window.
func (m Mark) ResetDefect(now time.Time) string {
	if reset, err := time.Parse(time.RFC3339, m.ResetAt); err == nil && reset.Sub(now) > MaxLimitHold {
		return fmt.Sprintf("mark resetAt %s is more than 5h ahead of now; ignored", m.ResetAt)
	}
	return ""
}

// StandingAt is Read with the horizon applied: a mark whose last
// feeding is older than Horizon has lapsed, unless a provider limit names
// a reset bounded by MaxLimitHold. A LastAt that does not
// parse lapses too — an unreadable age must not stand forever — and
// so does one more than Horizon in the FUTURE: a clock correction or
// a corrupt stamp must not pause the clocks beyond the same bound the
// horizon promises. A limit whose named reset has come lapses then,
// even inside the horizon. A reset more than five hours ahead is ignored.
func StandingAt(repoRoot string, now time.Time) (Mark, bool) {
	m, ok := Read(repoRoot)
	if !ok {
		return Mark{}, false
	}
	if m.ResetDefect(now) != "" {
		return m, false
	}
	last, err := time.Parse(time.RFC3339, m.LastAt)
	if err != nil {
		return Mark{}, false
	}
	horizon := Horizon
	if reset, err := time.Parse(time.RFC3339, m.ResetAt); err == nil {
		if !now.Before(reset) {
			return Mark{}, false
		}
		if m.LastClass == ProviderLimit {
			horizon = MaxLimitHold
		}
	}
	if age := now.Sub(last); age > horizon || age < -Horizon {
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
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("outage mark lock is held; the hint update is skipped: %w", err)
	}
	return &markLock{f: f}, nil
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
	m.ResetAt = ""
	if reset, ok := limitReset(class, detail, now); ok {
		if reset.Sub(now) > MaxLimitHold {
			reset = now.Add(MaxLimitHold)
		}
		m.ResetAt = reset.UTC().Format(time.RFC3339)
	}
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
	// The limit rules: the Claude CLI's usage-limit line ("Claude AI usage
	// limit reached|<epoch>", "5-hour limit reached ∙ resets 3pm", "You've
	// hit your usage limit", "You've hit your session limit, resets
	// 12:10am"), the API's own rate_limit_error token, and a
	// 429 under the same framing as the 5xx rule, so a count of 429 records
	// or a 429ms duration is nothing.
	limitWordRe = regexp.MustCompile(`(?i)(?:usage|5-hour|weekly|session) limit (?:reached|exceeded)|hit your (?:usage |session |weekly )?limit|(?:^|[^a-z_])rate_limit_error(?:[^a-z_]|$)`)
	limitCodeRe = regexp.MustCompile(`(?i)(?:^|[^a-z])(?:status|http|code|error)[^a-z0-9]{1,4}429(?:[^0-9a-z]|$)`)
)

// The reset a usage-limit line names: the Claude CLI's "resets 3pm" or
// "resets 3:30am (Europe/Amsterdam)", a clock time in the named zone or
// this machine's, and the epoch after "limit reached|".
var (
	resetClockRe = regexp.MustCompile(`(?i)\bresets (?:at )?(\d{1,2})(?::(\d{2}))?\s*(am|pm)(?:\s*\(([^)]+)\))?`)
	resetEpochRe = regexp.MustCompile(`(?i)limit reached\|(\d{9,11})\b`)
)

// limitReset is when a provider-limit failure seen at at says the limit
// resets: a named clock time is its next occurrence after at. A zone that
// does not load, or an epoch not after at, names no reset, so the mark keeps
// only its horizon.
func limitReset(class, detail string, at time.Time) (time.Time, bool) {
	if class != ProviderLimit {
		return time.Time{}, false
	}
	if match := resetEpochRe.FindStringSubmatch(detail); match != nil {
		seconds, err := strconv.ParseInt(match[1], 10, 64)
		reset := time.Unix(seconds, 0)
		return reset, err == nil && reset.After(at)
	}
	match := resetClockRe.FindStringSubmatch(detail)
	if match == nil {
		return time.Time{}, false
	}
	hour, _ := strconv.Atoi(match[1])
	minute := 0
	if match[2] != "" {
		minute, _ = strconv.Atoi(match[2])
	}
	if hour < 1 || hour > 12 || minute > 59 {
		return time.Time{}, false
	}
	hour %= 12
	if strings.EqualFold(match[3], "pm") {
		hour += 12
	}
	zone := time.Local
	if match[4] != "" {
		loaded, err := time.LoadLocation(strings.TrimSpace(match[4]))
		if err != nil {
			return time.Time{}, false
		}
		zone = loaded
	}
	local := at.In(zone)
	reset := time.Date(local.Year(), local.Month(), local.Day(), hour, minute, 0, 0, zone)
	if !reset.After(at) {
		reset = reset.AddDate(0, 0, 1)
	}
	return reset, true
}

// ResetRetryAt finds a reset within two minutes on either side of the
// message. A clock already passed today still permits one immediate retry.
func ResetRetryAt(class, detail string, at time.Time) (time.Time, bool) {
	reset, ok := limitReset(class, detail, at.Add(-2*time.Minute-time.Nanosecond))
	if !ok {
		return time.Time{}, false
	}
	distance := reset.Sub(at)
	return reset, distance >= -2*time.Minute && distance <= 2*time.Minute
}

// classifyLine names a line of provider-error evidence: an overload, a
// 5xx, or the provider's usage or rate limit; empty means neither.
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
	if limitWordRe.MatchString(line) || limitCodeRe.MatchString(line) {
		return ProviderLimit
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
