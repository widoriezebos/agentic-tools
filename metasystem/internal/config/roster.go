package config

// The rosters: each names, for every one of its rows, the runtime, model and
// effort that does one kind of agent work. The rosters are fixed: three for
// goal work, one per risk tier, and one each for the seat, the landing lane
// and the Project Partner. This file holds them, their types and their rows,
// and which roster and row each dispatched role and launch kind works from.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// The three ways a roster read refuses, recorded with a refused launch.
const (
	RosterUnset      = "LAUNCH_ROSTER_UNSET"
	RosterRuntime    = "LAUNCH_ROSTER_RUNTIME"
	RosterUnreadable = "LAUNCH_ROSTER_UNREADABLE"
)

type rosterKind struct {
	id, kind string
	rows     []string
}

var tierRows = []string{"design", "design-critique", "build", "code-critique", "verify", "investigate", "warden", "behavior-judge"}

// rosterKinds is the one table of rosters, their types and their rows. The
// identifiers are fixed; no other roster exists.
var rosterKinds = []rosterKind{
	{"tier-1", "tier", tierRows}, {"tier-2", "tier", tierRows}, {"tier-3", "tier", tierRows},
	{"seat", "seat", []string{"seat", "steward"}},
	{"landing", "landing", []string{"landing"}},
	{"partner", "partner", []string{"partner"}},
}

// The rows of dispatched roles and of launch kinds; each must be a row of
// the table, or RoleRow and LaunchRow answer that it has none.
var (
	roleRows   = map[string]string{"design-critic": "design-critique", "implementer": "build", "code-critic": "code-critique", "verifier": "verify", "investigator": "investigate", "warden": "warden", "behavior-judge": "behavior-judge", "steward-continuation": "steward"}
	launchRows = map[string]string{"design": "design", "critique": "design-critique", "build": "build", "read": "code-critique", "seat": "seat", "landing": "landing"}
	efforts    = []string{"low", "medium", "high", "xhigh", "max"}
	// mainRows are the rows no launch reads; the calling session does them.
	mainRows = []string{"verify", "investigate", "warden", "behavior-judge"}
	// authorReviewer pairs the row that does a piece of work with the row
	// that reviews it; within a roster the two name different models.
	authorReviewer = [][2]string{{"build", "code-critique"}, {"design", "design-critique"}}
)

// TierRoster is the roster a goal of the given risk tier works from. Work
// with no goal, or a tier outside 1 to 3, reads tier-3.
func TierRoster(tier uint8) string {
	if tier == 1 || tier == 2 {
		return fmt.Sprintf("tier-%d", tier)
	}
	return "tier-3"
}

// RoleRow is the roster and row a dispatched role works from; the
// implementer designs in mode design and builds in every other mode.
func RoleRow(role, mode string, tier uint8) (roster, row string, ok bool) {
	if role == "implementer" && mode == "design" {
		return rowOf("design", tier)
	}
	return rowOf(roleRows[role], tier)
}

// LaunchRow is the roster and row a launch kind starts its agent from.
func LaunchRow(kind string, tier uint8) (roster, row string, ok bool) {
	return rowOf(launchRows[kind], tier)
}

func rowOf(row string, tier uint8) (string, string, bool) {
	for _, kind := range rosterKinds {
		if slices.Contains(kind.rows, row) && (kind.kind != "tier" || kind.id == TierRoster(tier)) {
			return kind.id, row, true
		}
	}
	return "", "", false
}

// RosterAgent is what a row names; main names no model, the Partner no effort.
type RosterAgent struct {
	Runtime string `json:"runtime"`
	Model   string `json:"model,omitempty"`
	Effort  string `json:"effort,omitempty"`
}

type rosterEntry struct {
	Type string                 `json:"type"`
	Rows map[string]RosterAgent `json:"rows"`
}

// rostersFile is the file's form; a nil Version is a file that names none.
type rostersFile struct {
	Version *int                   `json:"version"`
	Rosters map[string]rosterEntry `json:"rosters"`
}

// RosterRefusal is a roster read that can't answer; Argv is what a person runs.
type RosterRefusal struct {
	Code, Message string
	Argv          []string
	Problem       string // why an unusable file can't be used
}

func (r *RosterRefusal) Error() string         { return r.Message }
func (r *RosterRefusal) RefusalCode() string   { return r.Code }
func (r *RosterRefusal) RefusalDetail() string { return r.Code + ": " + r.Message }

// RostersPath is the computer's rosters file under the registry home.
func RostersPath(home string) string { return filepath.Join(home, "host", "rosters.json") }

// RosterRow is the agent the computer's roster names for row, refused when
// the file is unusable, the row is not set, or its runtime is neither main
// nor in runtimes, the installation's list; nothing else is ever tried.
func RosterRow(home, roster, row string, runtimes []string) (RosterAgent, error) {
	file, problem := readRosters(home)
	if problem != "" {
		return RosterAgent{}, unusable(home, problem)
	}
	agent, ok := file.Rosters[roster].Rows[row]
	command := []string{"metasystem", "roster", "set", roster, row, "RUNTIME:MODEL:EFFORT"}
	if row == "partner" {
		command[5] = "RUNTIME:MODEL"
	}
	then := " A person sets it with: " + strings.Join(command, " ") + ". Nothing was started."
	switch {
	case !ok:
		return RosterAgent{}, &RosterRefusal{RosterUnset, "No agent is set for " + row + " in roster " + roster + " on this computer." + then, command, ""}
	case agent.Runtime != "main" && !slices.Contains(runtimes, agent.Runtime):
		return RosterAgent{}, &RosterRefusal{RosterRuntime, fmt.Sprintf("Roster %s runs %s on %s, and this installation runs %s.%s", roster, row, agent.Runtime, plainList(runtimes), then), command, ""}
	}
	return agent, nil
}

// RosterIDs are the rosters' identifiers, in the table's order.
func RosterIDs() (ids []string) {
	for _, kind := range rosterKinds {
		ids = append(ids, kind.id)
	}
	return ids
}

// RosterView is one roster as a person reads it: every row of its type, set or not.
type RosterView struct {
	ID   string       `json:"id"`
	Type string       `json:"type"`
	Rows []RosterLine `json:"rows"`
}

// RosterLine is one row of a roster; an unset row names no agent.
type RosterLine struct {
	Row string `json:"row"`
	Set bool   `json:"set"`
	RosterAgent
}

// Rosters answers every roster of the table, in its order. An unusable file
// is refused as RosterRow refuses it, so no row is ever answered from it.
func Rosters(home string) ([]RosterView, error) {
	file, problem := readRosters(home)
	if problem != "" {
		return nil, unusable(home, problem)
	}
	var views []RosterView
	for _, kind := range rosterKinds {
		view := RosterView{ID: kind.id, Type: kind.kind}
		for _, row := range kind.rows {
			agent, set := file.Rosters[kind.id].Rows[row]
			view.Rows = append(view.Rows, RosterLine{row, set, agent})
		}
		views = append(views, view)
	}
	return views, nil
}

// SetRosterRow writes one row of the computer's rosters, value being
// RUNTIME:MODEL:EFFORT, RUNTIME:MODEL for the Partner, or main; resolve is the
// model a runtime runs for a written one. A refused write leaves the file as
// it was; the file is read under the lock, so no writer loses a row another
// wrote meanwhile, and an unusable file is refused, never repaired.
func SetRosterRow(home, roster, row, value string, runtimes []string, resolve func(runtime, model string) (string, error)) (changed bool, err error) {
	i := slices.IndexFunc(rosterKinds, func(kind rosterKind) bool { return kind.id == roster })
	if i < 0 {
		return false, unchanged("%s is not a roster; the rosters are %s", roster, plainList(RosterIDs()))
	}
	if !slices.Contains(rosterKinds[i].rows, row) {
		return false, unchanged("%s is not a row of %s; its rows are %s", row, roster, plainList(rosterKinds[i].rows))
	}
	agent := RosterAgent{Runtime: "main"}
	if fields := strings.Split(value, ":"); value != "main" {
		// A value with more fields than a row has, or an empty one, is never
		// cut short to fit: the empty agent draws the row's form sentence.
		agent = RosterAgent{}
		if len(fields) <= 3 && !slices.Contains(fields, "") {
			fields = append(fields, "", "")
			agent = RosterAgent{Runtime: fields[0], Model: fields[1], Effort: fields[2]}
		}
	}
	if problem := rowProblem(row, agent); problem != "" {
		return false, unchanged("%s", problem)
	}
	if agent.Runtime != "main" && !slices.Contains(runtimes, agent.Runtime) {
		return false, unchanged("%s is not one of this installation's runtimes (%s)", agent.Runtime, strings.Join(runtimes, ", "))
	}
	if !filepath.IsAbs(home) {
		return false, unchanged("the rosters need an absolute home, got %q", home)
	}
	host := filepath.Dir(RostersPath(home))
	if err := os.MkdirAll(host, 0o700); err != nil {
		return false, err
	}
	held, err := lock.File(filepath.Join(host, "rosters.lock"), 0o600, lock.Exclusive)
	if err != nil {
		return false, err
	}
	defer held.Release()
	file, problem := readRosters(home)
	if problem != "" {
		return false, unchanged("the rosters file %s can't be used: %s", RostersPath(home), problem)
	}
	entry := file.Rosters[roster]
	if current, ok := entry.Rows[row]; ok && current == agent {
		return false, nil
	}
	// A reviewer on the author's model, even under an alias, shares the
	// author's blind spots, so the review would pass what the work got wrong.
	for _, pair := range authorReviewer {
		at := slices.Index(pair[:], row)
		if at < 0 || entry.Rows[pair[1-at]].Model == "" {
			continue
		}
		rows, runs, written := [2]RosterAgent{agent, agent}, [2]string{}, []string{}
		rows[1-at] = entry.Rows[pair[1-at]]
		for k, r := range rows {
			if runs[k], err = resolve(r.Runtime, r.Model); err != nil {
				return false, unchanged("can't tell which model %s runs for %s: %w", r.Runtime, r.Model, err)
			} else if runs[k] != r.Model {
				written = append(written, pair[k]+" is written "+r.Model)
			}
		}
		both := agent.Model
		if runs[0] != runs[1] {
			continue
		} else if rows[0].Model != rows[1].Model {
			both = runs[0] + " (" + plainList(written) + ")"
		}
		return false, unchanged("%s %s and %s would both run %s; the author and the reviewer of a piece of work are different models", roster, pair[0], pair[1], both)
	}
	if file.Rosters == nil {
		file.Rosters = map[string]rosterEntry{}
	}
	if entry.Rows == nil {
		entry.Rows = map[string]RosterAgent{}
	}
	version := 1
	entry.Type, entry.Rows[row] = rosterKinds[i].kind, agent
	file.Version, file.Rosters[roster] = &version, entry
	data, err := json.MarshalIndent(file, "", "  ")
	if err == nil {
		_, err = atomicfile.WriteFile(RostersPath(home), append(data, '\n'), 0o600, host)
	}
	return err == nil, err
}

// readRosters checks the whole file, so one broken row refuses every read.
func readRosters(home string) (file rostersFile, problem string) {
	const jsonSpace = " \t\r\n" // the white space JSON allows around a value
	data, err := os.ReadFile(RostersPath(home))
	if os.IsNotExist(err) {
		return file, ""
	} else if err != nil {
		return file, err.Error()
	}
	// The decoder takes null for an empty object and stops after one value.
	if !bytes.HasPrefix(bytes.TrimLeft(data, jsonSpace), []byte("{")) {
		return file, "it is not one JSON object"
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return file, "it is not valid rosters JSON (" + err.Error() + ")"
	}
	switch problem := keyProblem(json.NewDecoder(bytes.NewReader(data))); {
	case !utf8.Valid(data): // the decoder reads a broken byte in a string as U+FFFD
		return file, "it is not valid UTF-8"
	case len(bytes.Trim(data[decoder.InputOffset():], jsonSpace)) > 0:
		return file, "it holds more after its JSON object"
	case problem != "":
		return file, problem
	case file.Version == nil:
		return file, "it names no version, and this engine reads version 1"
	case *file.Version != 1:
		return file, fmt.Sprintf("its version is %d, and this engine reads version 1", *file.Version)
	}
	for _, id := range slices.Sorted(maps.Keys(file.Rosters)) {
		entry := file.Rosters[id]
		i := slices.IndexFunc(rosterKinds, func(kind rosterKind) bool { return kind.id == id })
		if i < 0 {
			return file, id + " is not a roster"
		} else if entry.Type != rosterKinds[i].kind {
			return file, fmt.Sprintf("roster %s has type %q, and its type is %s", id, entry.Type, rosterKinds[i].kind)
		}
		for _, row := range slices.Sorted(maps.Keys(entry.Rows)) {
			if problem := rowProblem(row, entry.Rows[row]); !slices.Contains(rosterKinds[i].rows, row) {
				return file, row + " is not a row of " + id
			} else if problem != "" {
				return file, id + " " + row + ": " + problem
			}
		}
	}
	return file, ""
}

// keyProblem is why the next value in decoder writes a key other than in lower
// case or twice in one object, which decoding would take for a field or let win.
func keyProblem(decoder *json.Decoder) string {
	open, err := decoder.Token()
	if err != nil {
		return err.Error()
	} else if open != json.Delim('{') && open != json.Delim('[') {
		return ""
	}
	for seen := map[string]bool{}; decoder.More(); {
		if open == json.Delim('{') {
			token, _ := decoder.Token()
			key, _ := token.(string)
			if lower := strings.ToLower(strings.ToUpper(key)); key != lower {
				return "it writes the key " + key + ", and keys are written in lower case: " + lower
			} else if seen[key] {
				return "it writes the key " + key + " twice in one object"
			}
			seen[key] = true
		}
		if problem := keyProblem(decoder); problem != "" {
			return problem
		}
	}
	decoder.Token() // the end of the object or array
	return ""
}

// rowProblem is why agent breaks a rule of row, or "". The installation's
// runtime list and the author-and-reviewer rule are the callers' to check.
func rowProblem(row string, agent RosterAgent) string {
	switch {
	case agent.Runtime == "main" && !slices.Contains(mainRows, row):
		return "main is only for verify, investigate, warden and behavior-judge"
	case agent.Runtime == "main" && agent.Model+agent.Effort != "":
		return "main takes no model or effort"
	case agent.Runtime == "main":
		return ""
	case agent.Runtime == AutoRuntime:
		return "auto is not a runtime; a row names this computer's agent"
	case row == "partner" && agent.Effort != "":
		return "the Partner takes no effort; write RUNTIME:MODEL"
	case row == "partner" && (agent.Runtime == "" || agent.Model == ""):
		return "the Partner's row is RUNTIME:MODEL"
	case row != "partner" && (agent.Runtime == "" || agent.Model == "" || !slices.Contains(efforts, agent.Effort)):
		return "a row is RUNTIME:MODEL:EFFORT, and effort is low, medium, high, xhigh or max"
	case templateValue.MatchString(agent.Model):
		return agent.Model + " is a template placeholder, not a model"
	}
	return ""
}

func unusable(home, problem string) error {
	return &RosterRefusal{RosterUnreadable, "The rosters file " + RostersPath(home) + " can't be used: " + problem + ". metasystem roster list shows what is wrong. Nothing was started.", []string{"metasystem", "roster", "list"}, problem}
}

func unchanged(format string, args ...any) error {
	return fmt.Errorf(format+"; nothing was changed", args...)
}

// plainList writes items as a person says them: "claude and codex".
func plainList(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
