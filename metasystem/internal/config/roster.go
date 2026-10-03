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
		return RosterAgent{}, &RosterRefusal{RosterUnset, "No agent is set for " + row + " in roster " + roster + " on this computer." + then, command}
	case agent.Runtime != "main" && !slices.Contains(runtimes, agent.Runtime):
		return RosterAgent{}, &RosterRefusal{RosterRuntime, fmt.Sprintf("Roster %s runs %s on %s, and this installation runs %s.%s", roster, row, agent.Runtime, plainList(runtimes), then), command}
	}
	return agent, nil
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
	return &RosterRefusal{RosterUnreadable, "The rosters file " + RostersPath(home) + " can't be used: " + problem + ". metasystem roster list shows what is wrong. Nothing was started.", []string{"metasystem", "roster", "list"}}
}

// plainList writes items as a person says them: "claude and codex".
func plainList(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}
