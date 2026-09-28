package delegation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/jsonedit"
)

var validIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// validID is dispatch.sh's valid_id.
func validID(id string) bool { return validIDPattern.MatchString(id) }

// recordOutcome is record_delegate_outcome: a compact typed outcome object
// with the four string fields, recorded only when the caller asked for one.
func (s *session) recordOutcome(outcome, headline, detail, job string) {
	if !s.env.RecordOutcome {
		return
	}
	line, err := jsonedit.Object([]string{"outcome=" + outcome, "headline=" + headline, "detail=" + detail, "jobId=" + job})
	if err != nil {
		return
	}
	s.outcome = []byte(line + "\n")
}

// recordOutcomeRaw is record_delegate_outcome_raw: an already encoded line.
func (s *session) recordOutcomeRaw(encoded string) {
	if !s.env.RecordOutcome {
		return
	}
	s.outcome = []byte(encoded + "\n")
}

// encodeJSON is the engine's printJSON encoding (json.Marshal, one line).
func encodeJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

// field reads a dotted field from a JSON file, rendered as json get renders
// it (strings bare, null as "null"). ok is false for an absent file, bad JSON
// or an absent field.
func field(path, name string) (string, bool) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return jsonedit.Get(content, name, nil)
}

// fieldOr is `json_field FILE FIELD 2>/dev/null || true`.
func fieldOr(path, name string) string {
	value, _ := field(path, name)
	return value
}

// valueField reads a dotted field from JSON text (json_value).
func valueField(content, name string) (string, bool) {
	return jsonedit.Get([]byte(content), name, nil)
}

// valueFieldOr is `json_value TEXT FIELD 2>/dev/null || true`.
func valueFieldOr(content, name string) string {
	value, _ := valueField(content, name)
	return value
}

// nullToEmpty is the `[[ "$x" == null ]] && x=` pattern.
func nullToEmpty(value string) string {
	if value == "null" {
		return ""
	}
	return value
}

func (s *session) nowISO() string {
	return s.l.ports.Clock.Now().UTC().Format("2006-01-02T15:04:05Z")
}

func (s *session) nowUnix() int64 { return s.l.ports.Clock.Now().Unix() }

func sha256File(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, handle); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// fixtureWaitCap is dispatch_fixture_wait_cap: a base wait scaled by the
// fixture cap scale, never below the base's ceiling rounding.
func (s *session) fixtureWaitCap(base int64) (int64, error) {
	scale := s.env.FixtureCapScaleMilli
	if scale == "" {
		scale = "1000"
	}
	milli, err := strconv.ParseInt(scale, 10, 64)
	if err != nil || milli < 1 || !positiveInteger(scale) || base < 1 {
		return 0, s.die(2, "dispatch wait cap inputs must be positive integers")
	}
	return (base*milli + 999) / 1000, nil
}

// pollInterval is milliseconds_to_sleep over an environment override.
func (s *session) pollInterval(defaultMS int64) (time.Duration, error) {
	raw := s.env.HandshakePollMS
	if raw == "" {
		return time.Duration(defaultMS) * time.Millisecond, nil
	}
	if !positiveInteger(raw) {
		return 0, s.die(2, "poll interval must be a positive integer in milliseconds")
	}
	value, _ := strconv.ParseInt(raw, 10, 64)
	return time.Duration(value) * time.Millisecond, nil
}

var positiveIntegerPattern = regexp.MustCompile(`^[1-9][0-9]*$`)

func positiveInteger(value string) bool { return positiveIntegerPattern.MatchString(value) }

var naturalPattern = regexp.MustCompile(`^[0-9]+$`)

// tempFile is mktemp: an empty file in dir (or TMPDIR when dir is empty)
// named after the pattern prefix.
func (s *session) tempFile(dir, prefix string) (string, error) {
	if dir == "" {
		dir = s.env.TempDir
	}
	if dir == "" {
		dir = os.TempDir()
	}
	handle, err := os.CreateTemp(dir, prefix+".*")
	if err != nil {
		return "", err
	}
	path := handle.Name()
	return path, handle.Close()
}

// mustTemp is a mktemp whose failure ends the command as set -e did.
func (s *session) mustTemp(dir, prefix string) (string, error) {
	path, err := s.tempFile(dir, prefix)
	if err != nil {
		s.eprintln("mktemp: " + err.Error())
		return "", exitWith(1)
	}
	return path, nil
}

// configGet is `metasystem-config.sh get --key KEY --default DEFAULT`.
func (s *session) configGet(key, def string) (string, error) {
	value, code, err := config.Get(config.GetParams{
		Key: key, Default: def, DefaultSet: true, ConfPath: filepath.Join(s.root, "metasystem.conf"),
	})
	if err != nil {
		s.eprintln(err.Error())
		if code == 0 {
			code = 1
		}
		return "", exitWith(code)
	}
	return value, nil
}

// goalNow is the engine's goal command clock: the fixture's semantic instant
// when the root authorizes one, else the lifecycle clock.
func (s *session) goalNow() (time.Time, error) {
	clock, _, err := s.goalClock()
	if err != nil {
		return time.Time{}, err
	}
	return clock(), nil
}

// goalClock is the engine's goal command clock over the lifecycle clock.
func (s *session) goalClock() (func() time.Time, bool, error) {
	clock := s.l.ports.Clock
	return fixtureauth.GoalClock(s.root, func() time.Time { return clock.Now().UTC() })
}

// verbFailure prints an owner error the way the job verbs' recordExit did
// and returns its exit code.
func (s *session) verbFailure(err error) error {
	if err == nil {
		return nil
	}
	var op *dispatch.OpError
	if asOpError(err, &op) {
		if op.Message != "" || op.Reason != "" {
			s.eprintln(op.Error())
		}
		return exitWith(op.Code)
	}
	s.eprintln(err.Error())
	return exitWith(1)
}

// verbCode is verbFailure's exit code without printing.
func verbCode(err error) int {
	if err == nil {
		return 0
	}
	var op *dispatch.OpError
	if asOpError(err, &op) {
		return op.Code
	}
	return 1
}

// removeQuietly is `rm -f -- PATH 2>/dev/null || true`.
func removeQuietly(path string) {
	if path != "" {
		_ = os.Remove(path)
	}
}

func copyFile(source, destination string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, content, 0o644)
}

func appendFile(path string, content []byte) error {
	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := handle.Write(content)
	closeErr := handle.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func touch(path string) error {
	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if err := handle.Close(); err != nil {
		return err
	}
	now := time.Now()
	return os.Chtimes(path, now, now)
}

func fileNonEmpty(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func (s *session) recordPath(job string) string { return filepath.Join(s.jobs, job+".json") }

func writePatch(path, body string) error {
	return os.WriteFile(path, []byte(body+"\n"), 0o600)
}
