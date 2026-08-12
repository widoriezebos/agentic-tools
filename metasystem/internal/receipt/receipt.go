package receipt

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/validate"
)

// The task-receipt ledger: add appends one receipt line at task completion, correct appends a correction that references an existing line
// without editing it, check decides whether a metasystem retro is due,
// stats prints the period numbers, and retro resets the cadence. The log is
// one line per record, so every free-text field is flattened before it is
// written.

// Options carries one invocation's inputs after flag parsing. Root is the
// checkout the shim resolves from; the retro cadence and the delegate
// validations resolve against it.
type Options struct {
	Root string
	File string

	Type        string
	Outcome     string
	Skills      string
	Verify      string
	Corrections string
	StopLoss    string
	Delegates   []string
	Note        string

	RefEpoch string
	RefSHA1  string
	Field    string
	Was      string
	NowValue string
	Reason   string

	Summary string
	All     bool

	MaxAgeDays     string
	MaxAgeSet      bool
	MaxReceipts    string
	MaxReceiptsSet bool

	Now       func() time.Time
	LookupEnv func(string) (string, bool) // defaults to os.LookupEnv
}

// Result is what an action prints and how it exits.
type Result struct {
	Out  []string
	Err  []string
	Code int
}

func fail(code int, format string, args ...any) Result {
	return Result{Err: []string{fmt.Sprintf(format, args...)}, Code: code}
}

func ok(lines ...string) Result { return Result{Out: lines} }

func (o *Options) now() time.Time {
	if o.Now == nil {
		return time.Now()
	}
	return o.Now()
}

// sanitize flattens the one field-corrupting class of input: newlines and
// carriage returns become spaces so the log stays one line per record.
func sanitize(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.ReplaceAll(value, "\n", " ")
}

func noPipes(value string) string { return strings.ReplaceAll(value, "|", ";") }

// readLines splits a log file the way the shell's read loop did: the final
// line counts even without a trailing newline.
func readLines(data string) []string {
	lines := strings.Split(data, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// cadence resolves one retro backstop through flag, environment,
// metasystem.conf, then the built-in default.
func (o *Options) cadence(key, flagValue string, flagSet bool, def string) (int64, *Result) {
	value, code, err := config.Get(config.GetParams{
		Key: key, Flag: flagValue, FlagSet: flagSet,
		Default: def, DefaultSet: true,
		ConfPath:  filepath.Join(o.Root, "metasystem.conf"),
		LookupEnv: o.LookupEnv,
	})
	if err != nil {
		r := fail(code, "%v", err)
		return 0, &r
	}
	n, convErr := strconv.ParseInt(value, 10, 64)
	if convErr != nil {
		r := fail(2, "invalid %s value: %s", key, value)
		return 0, &r
	}
	return n, nil
}

var (
	epochRe = regexp.MustCompile(`^[0-9]+$`)
	sha1Re  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	fieldRe = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// Add implements `receipt add`.
func Add(opts Options) Result {
	switch opts.Type {
	case "implement", "refactor", "improve", "review", "design", "investigate", "other":
	default:
		return fail(2, "invalid --type: %s", opts.Type)
	}
	switch opts.Outcome {
	case "shipped", "reworked", "blocked", "parked":
	default:
		return fail(2, "invalid --outcome: %s", opts.Outcome)
	}
	switch opts.Verify {
	case "clean", "caught", "skipped":
	default:
		return fail(2, "invalid --verify: %s", opts.Verify)
	}
	switch opts.StopLoss {
	case "yes", "no":
	default:
		return fail(2, "invalid --stop-loss: %s", opts.StopLoss)
	}
	if !epochRe.MatchString(opts.Corrections) {
		return fail(2, "invalid --corrections: %s", opts.Corrections)
	}
	if err := os.MkdirAll(filepath.Dir(opts.File), 0o755); err != nil {
		return fail(2, "cannot create receipt directory: %v", err)
	}
	skills := sanitize(opts.Skills)
	note := sanitize(opts.Note)
	delegate := "none"
	if len(opts.Delegates) > 0 {
		parts := make([]string, 0, len(opts.Delegates))
		for _, value := range opts.Delegates {
			parts = append(parts, noPipes(sanitize(value)))
		}
		delegate = strings.Join(parts, ",")
	}
	if strings.Contains(","+skills+",", ",code-critique,") {
		if !validate.CodeCritiqueClaim(opts.Root, opts.Delegates) {
			return fail(2, "receipt refused: skills=code-critique requires delegate entries naming a "+
				"code-critic chain id and the implementer job id in that chain's reviews field")
		}
	}
	class, stream := validate.WaiverFacts(opts.Root, opts.Delegates)
	class = noPipes(sanitize(class))
	stream = noPipes(sanitize(stream))
	now := opts.now().UTC()
	line := fmt.Sprintf("%d|%s|RECEIPT|type=%s|outcome=%s|skills=%s|verify=%s|corrections=%s|stop_loss=%s|delegate=%s|critique_waived=%s|waiver_stream=%s|note=%s\n",
		now.Unix(), now.Format("2006-01-02T15:04:05Z"), opts.Type, opts.Outcome, noPipes(skills), opts.Verify,
		opts.Corrections, opts.StopLoss, delegate, class, stream, noPipes(note))
	if err := appendLine(opts.File, line); err != nil {
		return fail(2, "cannot write receipt file: %v", err)
	}
	result := ok(fmt.Sprintf("receipt recorded in %s", opts.File))
	// The due-check after a write resolves cadence fresh from environment and
	// configuration; a cadence flag on add never reaches it.
	recheck := opts
	recheck.MaxAgeSet, recheck.MaxReceiptsSet = false, false
	recheck.MaxAgeDays, recheck.MaxReceipts = "", ""
	if after := Check(recheck); after.Code != 0 {
		result.Err = append(result.Err,
			"note: a metasystem retro is due — run skills/retro (scripts/receipt.sh check for details)")
	}
	return result
}

// Correct implements `receipt correct`.
func Correct(opts Options) Result {
	if !epochRe.MatchString(opts.RefEpoch) {
		return fail(2, "correct requires a numeric --ref-epoch")
	}
	if !sha1Re.MatchString(opts.RefSHA1) {
		return fail(2, "correct requires a lowercase 40-character --ref-sha1")
	}
	if !fieldRe.MatchString(opts.Field) {
		return fail(2, "correct requires a valid --field")
	}
	if opts.Reason == "" {
		return fail(2, "correct requires a nonempty --reason")
	}
	data, err := os.ReadFile(opts.File)
	if err != nil {
		return fail(2, "correction reference file does not exist: %s", opts.File)
	}
	was := noPipes(sanitize(opts.Was))
	nowValue := noPipes(sanitize(opts.NowValue))
	reason := noPipes(sanitize(opts.Reason))
	original := ""
	matches := 0
	for _, candidate := range readLines(string(data)) {
		prefix, _, _ := strings.Cut(candidate, "|")
		if prefix != opts.RefEpoch {
			continue
		}
		if fmt.Sprintf("%x", sha1.Sum([]byte(candidate))) != opts.RefSHA1 {
			continue
		}
		if !strings.Contains(candidate, "|RECEIPT|") {
			return fail(2, "correction reference must identify an original RECEIPT line")
		}
		original = candidate
		matches++
	}
	if matches != 1 {
		return fail(2, "correction reference must identify exactly one original line; matched %d", matches)
	}
	if !strings.Contains("|"+original+"|", "|"+opts.Field+"="+was+"|") {
		return fail(2, "correction --was value does not match field %s on the original line", opts.Field)
	}
	now := opts.now().UTC()
	line := fmt.Sprintf("%d|%s|CORRECTION|ref_epoch=%s|ref_sha1=%s|field=%s|was=%s|now=%s|reason=%s\n",
		now.Unix(), now.Format("2006-01-02T15:04:05Z"), opts.RefEpoch, opts.RefSHA1, opts.Field, was, nowValue, reason)
	if err := appendLine(opts.File, line); err != nil {
		return fail(2, "cannot write receipt file: %v", err)
	}
	return ok(fmt.Sprintf("correction recorded in %s; original line unchanged", opts.File))
}

// Retro implements `receipt retro`.
func Retro(opts Options) Result {
	if opts.Summary == "" {
		return fail(2, "retro requires a summary of the instruction changes made")
	}
	if err := os.MkdirAll(filepath.Dir(opts.File), 0o755); err != nil {
		return fail(2, "cannot create receipt directory: %v", err)
	}
	now := opts.now().UTC()
	line := fmt.Sprintf("%d|%s|RETRO|note=%s\n",
		now.Unix(), now.Format("2006-01-02T15:04:05Z"), noPipes(sanitize(opts.Summary)))
	if err := appendLine(opts.File, line); err != nil {
		return fail(2, "cannot write receipt file: %v", err)
	}
	return ok("retro recorded; cadence reset")
}

// Stats implements `receipt stats`.
func Stats(opts Options) Result {
	data, err := os.ReadFile(opts.File)
	if err != nil {
		return ok("receipts=0")
	}
	lines := readLines(string(data))
	if !opts.All {
		for i := len(lines) - 1; i >= 0; i-- {
			if strings.Contains(lines[i], "|RETRO|") {
				lines = lines[i+1:]
				break
			}
		}
	}
	var n, caught, stopLoss, waivers int
	var corrections, first, last int64
	outcomes := map[string]int{}
	types := map[string]int{}
	for _, line := range lines {
		fields := strings.Split(line, "|")
		if len(fields) < 3 || fields[2] != "RECEIPT" {
			continue
		}
		n++
		epoch := leadingInt(fields[0])
		if first == 0 {
			first = epoch
		}
		last = epoch
		for _, field := range fields[3:] {
			key, value, found := strings.Cut(field, "=")
			if !found {
				continue
			}
			switch key {
			case "outcome":
				outcomes[value]++
			case "type":
				types[value]++
			case "corrections":
				corrections += leadingInt(value)
			case "verify":
				if value == "caught" {
					caught++
				}
			case "stop_loss":
				if value == "yes" {
					stopLoss++
				}
			case "critique_waived":
				if value != "none" {
					waivers++
				}
			}
		}
	}
	out := []string{fmt.Sprintf("receipts=%d", n)}
	for _, outcome := range []string{"shipped", "reworked", "blocked", "parked"} {
		if count, present := outcomes[outcome]; present {
			out = append(out, fmt.Sprintf("outcome_%s=%d", outcome, count))
		}
	}
	for _, kind := range []string{"implement", "refactor", "improve", "review", "design", "investigate", "other"} {
		if count, present := types[kind]; present {
			out = append(out, fmt.Sprintf("type_%s=%d", kind, count))
		}
	}
	out = append(out,
		fmt.Sprintf("corrections=%d", corrections),
		fmt.Sprintf("caught_by_verify=%d", caught),
		fmt.Sprintf("stop_loss_triggered=%d", stopLoss),
		fmt.Sprintf("critique_waivers=%d", waivers))
	if n > 0 {
		out = append(out, fmt.Sprintf("span_days=%.1f", float64(last-first)/86400))
	}
	return ok(out...)
}

// Check implements `receipt check`.
func Check(opts Options) Result {
	data, err := os.ReadFile(opts.File)
	if err != nil {
		return ok("no receipts recorded yet")
	}
	lines := readLines(string(data))
	since := lines
	refEpoch := ""
	if len(lines) > 0 {
		refEpoch, _, _ = strings.Cut(lines[0], "|")
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], "|RETRO|") {
			since = lines[i+1:]
			refEpoch, _, _ = strings.Cut(lines[i], "|")
			break
		}
	}
	if !epochRe.MatchString(refEpoch) {
		return fail(2, "receipts file is malformed: %s", opts.File)
	}
	maxAgeDays, failed := opts.cadence("retro.max-age-days", opts.MaxAgeDays, opts.MaxAgeSet, "30")
	if failed != nil {
		return *failed
	}
	maxReceipts, failed := opts.cadence("retro.max-receipts", opts.MaxReceipts, opts.MaxReceiptsSet, "25")
	if failed != nil {
		return *failed
	}
	receipts := int64(0)
	for _, line := range since {
		if strings.Contains(line, "|RECEIPT|") {
			receipts++
		}
	}
	ref, _ := strconv.ParseInt(refEpoch, 10, 64)
	ageDays := (opts.now().UTC().Unix() - ref) / 86400
	if receipts == 0 {
		return ok("retro not due: no receipts this period, nothing to mine")
	}
	if receipts > maxReceipts {
		return fail(1, "metasystem retro due: %d receipts since the last retro (max %d)", receipts, maxReceipts)
	}
	if ageDays > maxAgeDays {
		return fail(1, "metasystem retro due: %d days since the last retro (max %d)", ageDays, maxAgeDays)
	}
	return ok(fmt.Sprintf("retro not due: %d receipts, %d days since last retro", receipts, ageDays))
}

func appendLine(path, line string) error {
	handle, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := handle.WriteString(line); err != nil {
		handle.Close()
		return err
	}
	return handle.Close()
}

// leadingInt reads the integer prefix of a value the way awk coerces a
// string to a number, so a malformed field degrades instead of erroring.
func leadingInt(value string) int64 {
	end := 0
	if end < len(value) && (value[end] == '-' || value[end] == '+') {
		end++
	}
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	n, _ := strconv.ParseInt(value[:end], 10, 64)
	return n
}
