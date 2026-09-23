// Package manifest is the interface describing itself, with one owner per
// fact.
//
// It has two halves and joins them at request time. The built half is composed
// at bundle time by the interface's own build, out of the registers the pages
// render from — the sections, the lanes, the help terms, the suggested
// questions — and travels inside the bundle as interface.json. The served half
// is composed here, every time it is asked for, out of the owners that live in
// Go: the acts with the hand each one needs, the `ui.` settings with their
// defaults and what this seat resolves them to, the record kinds with the
// homes the resolver puts them in for THIS checkout, and the runtimes this
// build will admit.
//
// The split is not tidiness. A record home is a path this checkout resolves,
// and a bundle built in one repository is served in another; a model default
// or an act's requirement can change in Go with no rebuild of the pages. So
// anything a build can freeze is frozen at build time, and anything that is
// true of a seat is read when it is asked for.
//
// Three statements about one thing are kept apart everywhere: what a thing is
// FOR, whether this build HAS it, and what the Project Partner may DO about
// it. Running them together is what teaches a Partner to send a human to a
// placeholder or to offer an edit it cannot make.
package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// SchemaVersion is the shape this package reads and the bundle script writes.
const SchemaVersion = 1

// BuiltPath is where the built half sits inside the served tree.
const BuiltPath = "interface.json"

// ErrNoBuilt reports that this executable carries no built half: a build with
// no bundle, or a bundle from before the interface described itself.
var ErrNoBuilt = errors.New("this build carries no interface manifest")

/* ------------------------------------------------------------ the halves -- */

// Built is the interface's own half, as the bundle carries it.
type Built struct {
	SchemaVersion int        `json:"schemaVersion"`
	Sections      []Section  `json:"sections"`
	Lanes         []Lane     `json:"lanes"`
	Terms         []Term     `json:"terms"`
	Questions     []Register `json:"questions"`
}

// Section is one destination of the rail.
type Section struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	// Purpose is what the section is for, from the help register.
	Purpose string `json:"purpose"`
	// Shows is what its page puts on the screen, from the section table.
	Shows string `json:"shows"`
	// Projected is whether this build renders it, from the same table that
	// decides what the shell routes.
	Projected bool `json:"projected"`
	// Availability says that in words, with the gate that brings an
	// unprojected one.
	Availability string `json:"availability"`
}

// Lane is one column of the board, whether or not this build shows it.
type Lane struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Purpose string `json:"purpose"`
	Shown   bool   `json:"shown"`
}

// Term is one word this interface uses in its own way.
type Term struct {
	ID   string `json:"id"`
	Term string `json:"term"`
	Text string `json:"text"`
}

// Register is the questions worth asking about one kind of subject.
type Register struct {
	Subject   string     `json:"subject"`
	Questions []Question `json:"questions"`
}

// Question is one suggested question and the set it is about.
type Question struct {
	Text  string `json:"text"`
	Scope string `json:"scope"`
}

/* ----------------------------------------------------------- the Go half -- */

// Act is one thing a human can do through this interface, and the hand it
// needs. The acts themselves are the server's; this is how they are described.
type Act struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Does is what the act changes.
	Does string `json:"does"`
	// Requires is the general requirement: what any caller must be for the
	// act to be admitted at all. It is not this request's eligibility, which
	// depends on who is asking and is answered when they ask.
	Requires string `json:"requires"`
}

// Setting is one `ui.` key, its default and what this seat resolves it to.
type Setting struct {
	Key     string `json:"key"`
	Purpose string `json:"purpose"`
	Default string `json:"default"`
	// Effective is the value on this seat, absent for a key whose name is a
	// family and for a value that must not leave the seat.
	Effective string `json:"effective,omitempty"`
	Family    bool   `json:"family,omitempty"`
}

// Kind is one kind of record the project keeps, and where it lives in THIS
// checkout, as the resolver puts it.
type Kind struct {
	Kind string `json:"kind"`
	// Purpose is the help register's own sentence for the kind, where the
	// interface shows one.
	Purpose string `json:"purpose,omitempty"`
	// Homes are the checkout-relative directories or files the resolver reads
	// this kind out of. A kind with more than one home carries them all: the
	// self-hosted layout keeps the kit's own designs beside the application's.
	Homes []string `json:"homes"`
	// Book marks a home whose index.md is itself a record carrying the
	// reading order; Register marks the one questions table.
	Book     bool `json:"book,omitempty"`
	Register bool `json:"register,omitempty"`
}

// Runtime is one agent this build will start as the Project Partner.
type Runtime struct {
	Name string `json:"name"`
	// Model is the model this build asks that runtime for when the seat names
	// none.
	Model string `json:"model"`
	// Command is the published entry point this build starts when the seat
	// names no command.
	Command string `json:"command"`
	// Configured says this seat named this runtime.
	Configured bool `json:"configured"`
	// Refusal is why a configured runtime was not admitted, in the
	// admission's own words.
	Refusal string `json:"refusal,omitempty"`
}

// Partner is what the Project Partner itself may do in this build. It is
// carried beside everything else so that "what this section is for" and "what
// I could do about it" are never read as one sentence.
type Partner struct {
	// Configured says a runtime answers as the Partner on this seat.
	Configured bool `json:"configured"`
	// Reads is what it may read.
	Reads string `json:"reads"`
	// Refused is what it may not do, whatever it is asked.
	Refused string `json:"refused"`
	// Writes is false in this build, and is a field rather than a sentence so
	// a reader can act on it without parsing prose.
	Writes bool `json:"writes"`
}

// Manifest is the two halves joined.
type Manifest struct {
	SchemaVersion int        `json:"schemaVersion"`
	Partner       Partner    `json:"partner"`
	Sections      []Section  `json:"sections"`
	Lanes         []Lane     `json:"lanes"`
	Terms         []Term     `json:"terms"`
	Questions     []Register `json:"questions"`
	Acts          []Act      `json:"acts"`
	Settings      []Setting  `json:"settings"`
	Records       []Kind     `json:"records"`
	Runtimes      []Runtime  `json:"runtimes"`
	// Problems are the halves this composition could not read, in the
	// reader's own words. A manifest says what it could not see rather than
	// leaving a reader to infer absence from silence.
	Problems []string `json:"problems,omitempty"`
}

/* --------------------------------------------------------- the composing -- */

// Sources are the owners this package composes the served half from. Each one
// is given rather than reached for, because the two processes that compose a
// manifest — the interface server and the tool server it hands the Partner —
// resolve their roots differently and neither may guess the other's.
type Sources struct {
	// Dist is the bundle's served tree, where the built half was written.
	// A nil tree is a build with no bundle, which the manifest says.
	Dist fs.FS
	// ConfPath is metasystem.conf, for resolving what this seat set.
	ConfPath string
	// Roots are the three roots the record resolver derives its homes from.
	Roots resolver.Roots
	// Acts are the acts this server offers, from the server that offers them.
	Acts []Act
	// Runtimes are the agents this build admits, from the package that
	// admits them.
	Runtimes []Runtime
	// Partner is what the Partner may do, from the package that tells it so.
	Partner Partner
}

// Compose joins the two halves. It never fails: a half it cannot read becomes
// a problem the manifest states, because a reader asking what this interface
// is made of must be able to tell "there is none" from "I could not look".
func Compose(sources Sources) Manifest {
	joined := Manifest{
		SchemaVersion: SchemaVersion,
		Partner:       sources.Partner,
		Acts:          sources.Acts,
		Runtimes:      sources.Runtimes,
		Settings:      settingsOf(sources.ConfPath),
	}
	built, err := ReadBuilt(sources.Dist)
	if err != nil {
		joined.Problems = append(joined.Problems,
			"the interface's own half of this manifest could not be read: "+err.Error())
	} else {
		joined.Sections = built.Sections
		joined.Lanes = built.Lanes
		joined.Terms = built.Terms
		joined.Questions = built.Questions
	}
	joined.Records = kindsOf(sources.Roots, joined.Terms)
	return joined
}

// ReadBuilt reads the built half out of a served tree.
func ReadBuilt(dist fs.FS) (Built, error) {
	if dist == nil {
		return Built{}, ErrNoBuilt
	}
	body, err := fs.ReadFile(dist, BuiltPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Built{}, ErrNoBuilt
		}
		return Built{}, err
	}
	var built Built
	if err := json.Unmarshal(body, &built); err != nil {
		return Built{}, fmt.Errorf("the interface manifest in this bundle is unparsable: %w", err)
	}
	if built.SchemaVersion != SchemaVersion {
		return Built{}, fmt.Errorf("the interface manifest in this bundle is version %d and this engine reads %d",
			built.SchemaVersion, SchemaVersion)
	}
	return built, nil
}

// settingsOf is the `ui.` register, resolved on this seat by the configuration
// reader. A family carries no effective value because its name is a pattern;
// a secret carries none because it must not leave the seat.
func settingsOf(confPath string) []Setting {
	read := config.ReadUISettings(confPath)
	settings := make([]Setting, 0, len(read))
	for _, one := range read {
		carried := Setting{Key: one.Key, Purpose: one.Purpose, Default: one.Default, Family: one.Family}
		if !one.Family && !one.Secret {
			carried.Effective = one.Value
		}
		settings = append(settings, carried)
	}
	return settings
}

// termForKind is which help term explains each kind of record. The words are
// the register's; this says which of them belongs to which kind, so that the
// two halves agree about a decision and a "Decisions".
var termForKind = map[string]string{
	resolver.KindIntent:   "intent",
	resolver.KindDoctrine: "doctrine",
	resolver.KindDecision: "decisions",
	resolver.KindDesign:   "designs",
	resolver.KindQuestion: "questions",
}

// kindsOf is every kind of record this project keeps, with the homes the
// resolver puts them in for this checkout. The homes are resolved rather than
// written down: the same bundle serves the template and an adopted
// application, and their homes are not the same paths.
func kindsOf(roots resolver.Roots, terms []Term) []Kind {
	said := map[string]string{}
	for _, term := range terms {
		said[term.ID] = term.Text
	}
	homes := map[string]*Kind{}
	order := []string{}
	for _, home := range resolver.Homes(roots) {
		kind := home.Kind
		if home.Register {
			kind = resolver.KindQuestion
		}
		known, seen := homes[kind]
		if !seen {
			known = &Kind{Kind: kind, Purpose: said[termForKind[kind]]}
			homes[kind] = known
			order = append(order, kind)
		}
		known.Homes = append(known.Homes, home.Rel)
		known.Book = known.Book || home.Book
		known.Register = known.Register || home.Register
	}
	kinds := make([]Kind, 0, len(order))
	for _, kind := range order {
		kinds = append(kinds, *homes[kind])
	}
	return kinds
}

/* ---------------------------------------------------------- the sections -- */

// The parts a reader can ask for one at a time. A manifest is longer than one
// bounded tool result, so the reader names a part and pages within it rather
// than being handed a truncated whole.
const (
	PartSummary   = "summary"
	PartSections  = "sections"
	PartLanes     = "lanes"
	PartTerms     = "terms"
	PartQuestions = "questions"
	PartActs      = "acts"
	PartSettings  = "settings"
	PartRecords   = "records"
	PartRuntimes  = "runtimes"
)

// Parts is every part, in the order the summary lists them.
var Parts = []string{
	PartSummary, PartSections, PartLanes, PartTerms, PartQuestions,
	PartActs, PartSettings, PartRecords, PartRuntimes,
}

// Lines renders one part as the lines a bounded reader pages through. An
// unknown part answers no lines and a false, so the caller can say what the
// parts are rather than answering an empty list.
func (m Manifest) Lines(part string) ([]string, bool) {
	switch strings.TrimSpace(part) {
	case "", PartSummary:
		return m.summary(), true
	case PartSections:
		return m.sectionLines(), true
	case PartLanes:
		return m.laneLines(), true
	case PartTerms:
		return m.termLines(), true
	case PartQuestions:
		return m.questionLines(), true
	case PartActs:
		return m.actLines(), true
	case PartSettings:
		return m.settingLines(), true
	case PartRecords:
		return m.recordLines(), true
	case PartRuntimes:
		return m.runtimeLines(), true
	default:
		return nil, false
	}
}

// summary is what a reader is given when it asks for nothing in particular:
// what the Partner may do, what this build projects and what it does not, and
// the parts it can ask for by name.
func (m Manifest) summary() []string {
	lines := []string{
		"- What the Project Partner may do here: read " + m.Partner.Reads + ".",
		"- What it may not do, whatever it is asked: " + m.Partner.Refused + ".",
		"- It cannot write. There is no tool in this build that would, and a request for one is refused before it runs.",
	}
	if !m.Partner.Configured {
		lines = append(lines, "- This seat has no Partner runtime configured.")
	}
	projected, absent := []string{}, []string{}
	for _, section := range m.Sections {
		if section.Projected {
			projected = append(projected, section.Title)
			continue
		}
		absent = append(absent, section.Title)
	}
	lines = append(lines,
		"- Sections this build projects ("+itoa(len(projected))+"): "+join(projected),
		"- Sections this build does not project ("+itoa(len(absent))+"): "+join(absent),
		"- Parts of this manifest, each read by naming it: "+strings.Join(Parts[1:], ", ")+".",
		"- Counts: "+itoa(len(m.Sections))+" sections, "+itoa(len(m.Lanes))+" lanes, "+
			itoa(len(m.Terms))+" terms, "+itoa(len(m.Questions))+" question registers, "+
			itoa(len(m.Acts))+" acts, "+itoa(len(m.Settings))+" settings, "+
			itoa(len(m.Records))+" record kinds, "+itoa(len(m.Runtimes))+" runtimes.",
	)
	for _, problem := range m.Problems {
		lines = append(lines, "- This manifest could not be read whole: "+problem)
	}
	return lines
}

func (m Manifest) sectionLines() []string {
	lines := make([]string, 0, len(m.Sections))
	for _, section := range m.Sections {
		lines = append(lines, "- "+section.Title+" ("+section.ID+", at "+section.Path+")\n"+
			"  - For: "+section.Purpose+"\n"+
			"  - Shows: "+section.Shows+"\n"+
			"  - In this build: "+section.Availability)
	}
	if len(lines) == 0 {
		lines = append(lines, "- This build could not read the interface's own half of the manifest, so it cannot name its sections.")
	}
	return lines
}

func (m Manifest) laneLines() []string {
	lines := make([]string, 0, len(m.Lanes))
	for _, lane := range m.Lanes {
		shown := "the board does not show this lane in this build"
		if lane.Shown {
			shown = "shown on the board"
		}
		lines = append(lines, "- "+lane.Title+" ("+lane.ID+"): "+lane.Purpose+" — "+shown)
	}
	return lines
}

func (m Manifest) termLines() []string {
	lines := make([]string, 0, len(m.Terms))
	for _, term := range m.Terms {
		lines = append(lines, "- "+term.Term+" ("+term.ID+"): "+term.Text)
	}
	return lines
}

func (m Manifest) questionLines() []string {
	lines := make([]string, 0, len(m.Questions))
	for _, register := range m.Questions {
		asked := make([]string, 0, len(register.Questions))
		for _, question := range register.Questions {
			asked = append(asked, "\n  - \""+question.Text+"\" — about "+question.Scope)
		}
		lines = append(lines, "- About a "+register.Subject+", this interface suggests:"+strings.Join(asked, ""))
	}
	return lines
}

func (m Manifest) actLines() []string {
	lines := make([]string, 0, len(m.Acts))
	for _, act := range m.Acts {
		lines = append(lines, "- "+act.Title+" ("+act.ID+"): "+act.Does+" Requires "+act.Requires+".")
	}
	if len(lines) == 0 {
		return []string{"- This build offers no acts."}
	}
	return append(lines,
		"- The Project Partner performs none of these. They are the human's, through the interface, and nothing the Partner sends can become one.")
}

func (m Manifest) settingLines() []string {
	lines := make([]string, 0, len(m.Settings))
	for _, setting := range m.Settings {
		line := "- " + setting.Key + ": " + setting.Purpose + " Default: " + quoted(setting.Default) + "."
		switch {
		case setting.Family:
			line += " The last segment is a runtime's name, so this seat resolves one key per runtime rather than one value."
		default:
			line += " On this seat: " + quoted(setting.Effective) + "."
		}
		lines = append(lines, line)
	}
	return lines
}

func (m Manifest) recordLines() []string {
	lines := make([]string, 0, len(m.Records))
	for _, kind := range m.Records {
		line := "- " + kind.Kind + ": "
		if kind.Purpose != "" {
			line += kind.Purpose + " "
		}
		line += "Read in this checkout from " + join(kind.Homes) + "."
		if kind.Book {
			line += " Its home's index.md is itself a record, carrying the reading order."
		}
		if kind.Register {
			line += " It is a table of rows rather than a page each."
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{"- This build could not resolve where this checkout keeps its records."}
	}
	return lines
}

func (m Manifest) runtimeLines() []string {
	lines := make([]string, 0, len(m.Runtimes))
	for _, runtime := range m.Runtimes {
		line := "- " + runtime.Name + ": started as " + quoted(runtime.Command) +
			", asked for the model " + quoted(runtime.Model) + " when this seat names none."
		if runtime.Configured {
			line += " This seat names it as its Partner."
		}
		if runtime.Refusal != "" {
			line += " It was not admitted: " + runtime.Refusal + "."
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{"- This build admits no Partner runtime."}
	}
	return lines
}

func quoted(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unset"
	}
	return "\"" + value + "\""
}

func join(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func itoa(count int) string { return fmt.Sprintf("%d", count) }
