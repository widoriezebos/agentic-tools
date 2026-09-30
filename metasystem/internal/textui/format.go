// Package textui lays out what a metasystem command prints for a person:
// a headline, sections of aligned rows, one hint, colour on a terminal
// only, and one vocabulary of states, times, paths and numbers
// (plans/designs/output-style.md, "Output style"). It imports nothing of
// the metasystem, so every owner that still formats a line can use it.
package textui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MaxWidth is the widest line any command prints (P9).
const MaxWidth = 100

// Env is everything layout depends on; tests build it by hand.
type Env struct {
	Width   int            // effective width, at most MaxWidth
	Color   bool           // P8: a terminal, NO_COLOR empty, TERM not dumb
	ASCII   bool           // §3: the locale cannot render the symbols
	TTY     bool           // the stream is a terminal: legacy lines wrap
	Verbose bool           // --verbose: callers add verbose-only blocks
	Now     time.Time      // the invocation's clock
	Zone    *time.Location // the local zone times are told in
	Home    string         // for ~
	Repo    string         // the checkout root, for repo-relative paths
	InRepo  bool           // the working directory is inside Repo
}

// Detect reads the stream (a terminal or not, its width), COLUMNS,
// NO_COLOR, TERM and the locale.
func Detect(fd uintptr, getenv func(string) string, now time.Time, zone *time.Location) Env {
	tty := IsTerminal(fd)
	columns := 0
	if tty {
		columns = TerminalWidth(fd)
	}
	return DetectWith(tty, columns, getenv, now, zone)
}

// DetectWith is Detect with the stream's answers given: whether it is a
// terminal and the terminal's width (0 when unknown).
func DetectWith(tty bool, termWidth int, getenv func(string) string, now time.Time, zone *time.Location) Env {
	if zone == nil {
		zone = time.Local
	}
	width := MaxWidth
	if columns, err := strconv.Atoi(strings.TrimSpace(getenv("COLUMNS"))); err == nil && columns > 0 {
		width = columns
	} else if tty && termWidth > 0 {
		width = termWidth
	}
	width = min(width, MaxWidth)
	color := tty && getenv("NO_COLOR") == "" && getenv("TERM") != "dumb"
	return Env{Width: width, Color: color, ASCII: asciiLocale(getenv), TTY: tty, Now: now, Zone: zone}
}

// asciiLocale: METASYSTEM_ASCII=1, or the first locale variable set names
// a character set other than UTF-8. No locale at all keeps the symbols.
func asciiLocale(getenv func(string) string) bool {
	if getenv("METASYSTEM_ASCII") == "1" {
		return true
	}
	for _, key := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
		if value := getenv(key); value != "" {
			lower := strings.ToLower(value)
			return !strings.Contains(lower, "utf-8") && !strings.Contains(lower, "utf8")
		}
	}
	return false
}

func (e Env) zone() *time.Location {
	if e.Zone == nil {
		return time.Local
	}
	return e.Zone
}

// dayDistance is the number of calendar days from the invocation's day to
// t's, in the local zone: 0 today, 1 tomorrow, -1 yesterday.
func (e Env) dayDistance(t time.Time) int {
	day := func(at time.Time) int64 {
		local := at.In(e.zone())
		return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC).Unix() / 86400
	}
	return int(day(t) - day(e.Now))
}

// Time is a local time as short as its distance allows (P5): 10:53 today,
// Tue 05:43 within six days, 28 Sep 12:39 this year, 2025-09-28 otherwise.
func (e Env) Time(t time.Time) string {
	local := t.In(e.zone())
	days := e.dayDistance(t)
	switch {
	case days == 0:
		return local.Format("15:04")
	case days >= -6 && days <= 6:
		return local.Format("Mon 15:04")
	case local.Year() == e.Now.In(e.zone()).Year():
		return local.Format("2 Jan 15:04")
	}
	return local.Format("2006-01-02")
}

// Since is a start: since 10:53.
func (e Env) Since(t time.Time) string { return "since " + e.Time(t) }

// Ago is a sighting: 30s ago, 10m ago, 18h ago, 3d ago; a time still ahead
// is in 5m.
func (e Env) Ago(t time.Time) string {
	d := e.Now.Sub(t)
	if d < 0 {
		return "in " + agoUnit(-d)
	}
	return agoUnit(d) + " ago"
}

func agoUnit(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	}
	return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
}

// Until is an end with the time left: until tomorrow 10:56 (23h54m left);
// one already past says it ended.
func (e Env) Until(t time.Time) string {
	var when string
	switch days := e.dayDistance(t); {
	case days == 1:
		when = "tomorrow " + t.In(e.zone()).Format("15:04")
	default:
		when = e.Time(t)
	}
	left := t.Sub(e.Now)
	if left <= 0 {
		return "until " + when + " (ended)"
	}
	return "until " + when + " (" + Duration(left) + " left)"
}

// Path is repo-relative inside the repository, ~/… elsewhere under the
// home directory, and as given otherwise (P6).
func (e Env) Path(path string) string {
	if path == "" {
		return path
	}
	if e.Repo != "" && strings.HasPrefix(path, strings.TrimSuffix(e.Repo, "/")+"/") {
		return strings.TrimPrefix(path, strings.TrimSuffix(e.Repo, "/")+"/")
	}
	home := strings.TrimSuffix(e.Home, "/")
	switch {
	case home == "":
		return path
	case path == home:
		return "~"
	case strings.HasPrefix(path, home+"/"):
		return "~/" + strings.TrimPrefix(path, home+"/")
	}
	return path
}

// Duration is 45s, 12m, 2h05m, 61h04m under 72 hours, 3d04h above; a
// negative duration is told by its size.
func Duration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d/time.Second))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 72*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d/time.Hour), int(d%time.Hour/time.Minute))
	}
	return fmt.Sprintf("%dd%02dh", int(d/(24*time.Hour)), int(d%(24*time.Hour)/time.Hour))
}

// Bytes is a size in IEC units with one decimal: 231.4 GiB, 25.7 MiB,
// 20.0 KiB, 0 B.
func Bytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// BytesMiB is Bytes without the KiB step: a size under a MiB in bytes. It
// is the form disk's summaries carry, so their --json stays as it is.
func BytesMiB(n int64) string {
	if n < 1<<20 {
		return fmt.Sprintf("%d B", n)
	}
	return Bytes(n)
}

// GiB is a size in GiB with two decimals, the form evidence's records and
// summaries carry, so their --json stays as it is.
func GiB(n int64) string { return fmt.Sprintf("%.2f GiB", float64(n)/float64(1<<30)) }

// Count is a counted noun: no goals, 1 goal, 3,273 goals (P7).
func Count(n int, one, many string) string {
	switch n {
	case 0:
		return "no " + many
	case 1:
		return "1 " + one
	}
	return Number(int64(n)) + " " + many
}

// Number groups thousands: 3,273.
func Number(n int64) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if n < 0 {
		sign, digits = "-", digits[1:]
	}
	var out strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return sign + out.String()
}

// SHA is a commit as this repository's git log --oneline shows it: nine
// characters, or as stored when the record carries fewer.
func SHA(s string) string {
	if len(s) > 9 {
		return s[:9]
	}
	return s
}

// Command is an argument vector as a person pastes it: a safe word bare,
// any other single-quoted.
func Command(argv []string) string {
	quoted := make([]string, len(argv))
	for index, arg := range argv {
		if arg != "" && strings.IndexFunc(arg, func(r rune) bool {
			return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("_@%+=:,./-", r))
		}) == -1 {
			quoted[index] = arg
		} else {
			quoted[index] = "'" + strings.ReplaceAll(arg, "'", `'"'"'`) + "'"
		}
	}
	return strings.Join(quoted, " ")
}

// Wrap breaks text at word boundaries into lines of at most width columns;
// every line after the first starts with indent spaces. A word longer than
// the room left gets a line of its own and is never broken.
func Wrap(text string, width, indent int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current, pad := "", ""
	for _, word := range words {
		switch {
		case current == "":
			current = pad + word
		case width > 0 && utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) > width:
			lines = append(lines, current)
			pad = strings.Repeat(" ", indent)
			current = pad + word
		default:
			current += " " + word
		}
	}
	return append(lines, current)
}
