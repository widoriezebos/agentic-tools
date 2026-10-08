// Package board is the host board: one progress card per claimed goal, written
// by the owner of each stage transition in the same act as its own record,
// and read for the armed seats a caller names (batch-lane design D14, R24).
//
// A card is identifiers, numbers and times, never free text, and nothing in
// it names a language. The board is the user's alone: every directory it
// forms is created and checked here with 0700, every path component is one
// word of [A-Za-z0-9._-], and every card is published 0600 through the
// atomic writer. The package imports only the standard library, atomicfile
// and identity, so every stage owner (dispatch, launch, goal, the lane) can
// import it without a cycle.
package board

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// SchemaVersion is the card schema this engine writes; a reader that meets a
// higher number reads the card as Unknown.
const SchemaVersion = 1

// registryHomeEnv redirects the board exactly as it redirects the host's
// armed-checkouts registry (registry.DefaultPath), so a fixture moves both.
const registryHomeEnv = "METASYSTEM_SUPERVISION_REGISTRY_HOME"

// enrollmentRemedy is the command that gives a seat a safe nickname.
const enrollmentRemedy = "git config metasystem.goal.machine <nickname> (one word of letters, digits, '.', '_' and '-')"

// Stage is where a claimed goal stands.
type Stage string

const (
	StageClaimedIdle Stage = "claimed-idle"
	StageBuild       Stage = "build"
	StageRevise      Stage = "revise"
	StageUnitProof   Stage = "unit-proof"
	StageReview      Stage = "review"
	StageJudgement   Stage = "judgement"
	StageLandReady   Stage = "land-ready"
	StageJoined      Stage = "joined"
	StageLanding     Stage = "landing"
	StageLanded      Stage = "landed"
	StageReturned    Stage = "returned"
	StageReleased    Stage = "released"
)

var stages = map[Stage]bool{
	StageClaimedIdle: false, StageBuild: false, StageRevise: false, StageUnitProof: false,
	StageReview: false, StageJudgement: false, StageLandReady: false, StageJoined: false,
	StageLanding: false, StageLanded: true, StageReturned: true, StageReleased: true,
}

// Valid reports whether the stage is one of the vocabulary.
func (s Stage) Valid() bool { _, ok := stages[s]; return ok }

// Terminal reports whether the stage ends the claim's life on the board.
func (s Stage) Terminal() bool { return stages[s] }

// ProcessBound reports whether a live process carries the stage, so the card
// must name it as owner.
func (s Stage) ProcessBound() bool {
	switch s {
	case StageBuild, StageRevise, StageUnitProof, StageReview, StageLanding:
		return true
	}
	return false
}

// progressing reports whether the stage is expected to move, so a card whose
// last real progress is older than the stall bound is disbelieved. A claim
// with no work, a joined unit and a terminal card wait on nothing.
func (s Stage) progressing() bool {
	return s.Valid() && !s.Terminal() && s != StageClaimedIdle && s != StageJoined
}

// Seat names one armed checkout: its enrolled nickname and its installation
// root (<checkout>/metasystem).
type Seat struct {
	Machine      string `json:"machine"`
	Installation string `json:"installation"`
}

// Round is the round of its limit; Max is nil for a build without a limit.
type Round struct {
	N   int  `json:"n"`
	Max *int `json:"max"`
}

// Job is the delegate job or the launch that carries the stage.
type Job struct {
	ID    string `json:"id"`
	Role  string `json:"role,omitempty"`
	Phase string `json:"phase,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// Proof is a proof run's structural progress: Done distinct planned sections
// ended of Planned.
type Proof struct {
	Attempt string `json:"attempt"`
	Done    int    `json:"done"`
	Planned int    `json:"planned"`
}

// Owner is the process the stage's life depends on, never a caller that
// returns while the work runs. PidStartedAt is the process start in Unix
// seconds, the job record's pidStartedAt.
type Owner struct {
	Pid          int64 `json:"pid"`
	PidStartedAt int64 `json:"pidStartedAt"`
}

// StageSpan is one closed stage of a claim: the history estimates come from.
type StageSpan struct {
	Stage Stage     `json:"stage"`
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
}

// Writer is the provenance of a card's last write.
type Writer struct {
	Pid          int64     `json:"pid"`
	PidStartedAt int64     `json:"pidStartedAt"`
	Component    string    `json:"component"`
	At           time.Time `json:"at"`
}

// Card is one claimed goal's progress on one seat.
// ReviewStop projects the unit owner's decision; it grants no permission.
type ReviewStop struct {
	Decision string `json:"decision"`
	Handoff  string `json:"handoff"`
	Class    string `json:"class"`
	Attempt  int    `json:"attempt"`
	Budget   int    `json:"budget"`
}

type Card struct {
	Stop           *ReviewStop `json:"stop,omitempty"`
	SchemaVersion  int         `json:"schemaVersion"`
	Seat           Seat        `json:"seat"`
	Goal           string      `json:"goal"`
	Stage          Stage       `json:"stage"`
	Round          *Round      `json:"round,omitempty"`
	Job            *Job        `json:"job,omitempty"`
	Proof          *Proof      `json:"proof,omitempty"`
	Since          time.Time   `json:"since"`
	LastProgressAt time.Time   `json:"lastProgressAt"`
	Owner          *Owner      `json:"owner,omitempty"`
	Batch          string      `json:"batch,omitempty"`
	Landed         int         `json:"landed,omitempty"`
	Stages         []StageSpan `json:"stages,omitempty"`
	Writer         Writer      `json:"writer"`
}

// Home is the directory the board lives under: the registry's own
// .metasystem directory, redirected by the registry's fixture variable.
func Home() (string, error) { return HomeWith(os.LookupEnv) }

// HomeWith is Home read through lookup: a hook resolves the board from the
// environment its invocation carries.
func HomeWith(lookup func(string) (string, bool)) (string, error) {
	if override, _ := lookup(registryHomeEnv); override != "" {
		if !filepath.IsAbs(override) {
			return "", fmt.Errorf("%s must name an absolute run-scoped home", registryHomeEnv)
		}
		return filepath.Join(override, ".metasystem"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".metasystem"), nil
}

// Dir is the board directory under home.
func Dir(home string) string { return filepath.Join(home, "host", "board") }

// safeComponent is the presence writer's nickname rule (seat.ValidateMachineName),
// held again here because board imports neither seat nor goal. It covers goal
// ids, which are [a-z0-9._-]{1,128}.
var safeComponent = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// SafeName reports whether name may be one path component of the board.
func SafeName(name string) bool {
	return safeComponent.MatchString(name) && name != "." && name != ".."
}

// checkName refuses a nickname or goal that could leave the board.
func checkName(kind, name string) error {
	if SafeName(name) {
		return nil
	}
	return coded("BOARD_NAME_UNSAFE", kind+"="+name, fmt.Errorf("no card was written: the %s %q is not one word of letters, digits, '.', '_' and '-'\nenroll a safe nickname: %s", kind, name, enrollmentRemedy))
}

// privateDir makes dir a private directory: created 0700 when absent, a real
// directory and never a symlink when present, tightened to 0700 when more
// permissive. The atomic writer never creates a board directory (it would
// make it 0755).
func privateDir(dir string) error {
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err = os.Lstat(dir)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return notADirectory(dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return os.Chmod(dir, 0o700)
	}
	return nil
}

// seatDir checks every component from home down and returns the seat's
// private directory, creating host/, host/board/ and board/<seat>/ as needed.
func seatDir(home, machine string) (string, error) {
	if err := checkName("seat nickname", machine); err != nil {
		return "", err
	}
	dir, err := boardDir(home)
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, machine)
	if err := privateDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// boardDir checks every component from home down and returns the private
// board directory, creating host/ and host/board/ as needed.
func boardDir(home string) (string, error) {
	info, err := os.Lstat(home)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(home, 0o700); err != nil {
			return "", err
		}
		info, err = os.Lstat(home)
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return "", notADirectory(home)
	}
	dir := home
	for _, component := range []string{"host", "board"} {
		dir = filepath.Join(dir, component)
		if err := privateDir(dir); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// Write publishes card on the board under Home.
func Write(card Card) error {
	home, err := Home()
	if err != nil {
		return err
	}
	return WriteAt(home, card)
}

// WriteAt publishes card as its seat's card for its goal under home, in one
// atomic replace under the seat's lock. The writer supplies the stage and its
// markers; the board carries the claim's history: a stage change closes the
// previous stage into Stages and stamps real progress; the same stage keeps
// its Since and advances LastProgressAt only on real progress (a new phase,
// round or job, or more proof sections done), so a repeated event moves
// nothing. A card after a terminal one starts a new claim's history.
func WriteAt(home string, card Card) error {
	if err := checkName("goal", card.Goal); err != nil {
		return err
	}
	if !card.Stage.Valid() {
		return fmt.Errorf("board: card for goal %s names no stage of the vocabulary: %q", card.Goal, card.Stage)
	}
	return Update(home, card.Seat, card.Goal, func(Card) (Card, bool) { return card, true })
}

// Update reads, decides and publishes a goal's card under its seat's lock.
// A missing or unreadable card is zero; a false decision leaves it untouched.
func Update(home string, seat Seat, goal string, decide func(Card) (Card, bool)) error {
	if err := checkName("goal", goal); err != nil {
		return err
	}
	dir, err := seatDir(home, seat.Machine)
	if err != nil {
		return err
	}
	unlock, err := lockSeat(dir)
	if err != nil {
		return err
	}
	defer unlock()
	path := filepath.Join(dir, goal+".json")
	previous, havePrevious := readCardFile(path)
	card, write := decide(previous)
	if !write {
		return nil
	}
	card.Seat, card.Goal = seat, goal
	if !card.Stage.Valid() {
		return fmt.Errorf("board: card for goal %s names no stage of the vocabulary: %q", goal, card.Stage)
	}
	next := carry(card, previous, havePrevious)
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(data)+"\n", home)
	return err
}

// carry fills what the board owns: schema, provenance, stage times and the
// closed spans of the claim.
func carry(card Card, previous Card, havePrevious bool) Card {
	card.SchemaVersion = SchemaVersion
	if card.Writer.At.IsZero() {
		card.Writer.At = time.Now()
	}
	at := card.Writer.At.UTC()
	card.Writer.At = at
	card.Writer.Pid, card.Writer.PidStartedAt = self()
	if !card.Since.IsZero() {
		card.Since = card.Since.UTC()
	}
	sameClaim := havePrevious && !previous.Stage.Terminal() && previous.Seat.Installation == card.Seat.Installation
	if !sameClaim {
		card.Stages = nil
		if card.Since.IsZero() {
			card.Since = at
		}
		card.LastProgressAt = at
		return card
	}
	card.Stages = append([]StageSpan(nil), previous.Stages...)
	if card.Landed == 0 {
		card.Landed = previous.Landed
	}
	if previous.Stage != card.Stage {
		if card.Since.IsZero() {
			card.Since = at
		}
		card.Stages = append(card.Stages, StageSpan{Stage: previous.Stage, Since: previous.Since, Until: card.Since})
		card.LastProgressAt = at
		return card
	}
	if card.Since.IsZero() {
		card.Since = previous.Since
	}
	card.LastProgressAt = previous.LastProgressAt
	if progressed(previous, card) {
		card.LastProgressAt = at
	}
	return card
}

// progressed is the one rule of real progress within a stage.
func progressed(previous, next Card) bool {
	if jobKey(previous.Job) != jobKey(next.Job) || roundKey(previous.Round) != roundKey(next.Round) {
		return true
	}
	if next.Proof != nil {
		if previous.Proof == nil || previous.Proof.Attempt != next.Proof.Attempt || next.Proof.Done > previous.Proof.Done {
			return true
		}
	}
	return previous.Batch != next.Batch || previous.Landed != next.Landed
}

func jobKey(job *Job) string {
	if job == nil {
		return ""
	}
	return strings.Join([]string{job.ID, job.Role, job.Phase, job.Kind}, "\x00")
}

func roundKey(round *Round) string {
	if round == nil {
		return ""
	}
	if round.Max == nil {
		return fmt.Sprintf("%d/-", round.N)
	}
	return fmt.Sprintf("%d/%d", round.N, *round.Max)
}

var selfIdentity = sync.OnceValues(func() (int64, int64) {
	pid := int64(os.Getpid())
	exact, state, err := (identity.KernelProber{}).Probe(pid)
	if err != nil || state != identity.Alive {
		return pid, 0
	}
	return pid, exact.StartedAt.Unix()
})

// self is this process's identity, read once.
func self() (int64, int64) { return selfIdentity() }

// Self is this process as a card's owner: a stage owner whose own process
// carries the stage names itself with it.
func Self() *Owner {
	pid, started := self()
	return &Owner{Pid: pid, PidStartedAt: started}
}

// lockSeat serializes the read-modify-write of one seat's cards across
// processes. The lock file is never removed.
func lockSeat(dir string) (func(), error) {
	file, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

// readCardFile reads one card; false when it is absent or malformed.
func readCardFile(path string) (Card, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Card{}, false
	}
	var card Card
	if json.Unmarshal(data, &card) != nil {
		return Card{}, false
	}
	return card, true
}

// LiveCard is the one non-terminal card for goal on the board under home, on
// whichever seat holds it: a stage owner that knows the goal but not the
// seat (the proof-run launcher) advances the card the claim's seat wrote.
// False when no seat, or more than one, holds a live card for the goal.
func LiveCard(home, goal string) (Card, bool) {
	return claimCard(home, goal, false)
}

// LiveOrReturnedCard finds a claim that can join the lane again after a return.
func LiveOrReturnedCard(home, goal string) (Card, bool) {
	if card, live := LiveCard(home, goal); live {
		return card, true
	}
	return claimCard(home, goal, true)
}

func claimCard(home, goal string, returned bool) (Card, bool) {
	if !SafeName(goal) {
		return Card{}, false
	}
	entries, err := os.ReadDir(Dir(home))
	if err != nil {
		return Card{}, false
	}
	var found []Card
	for _, entry := range entries {
		if !entry.IsDir() || !SafeName(entry.Name()) {
			continue
		}
		card, ok := readCardFile(filepath.Join(Dir(home), entry.Name(), goal+".json"))
		if ok && card.Goal == goal && card.Seat.Machine == entry.Name() && (!card.Stage.Terminal() || returned && card.Stage == StageReturned) {
			found = append(found, card)
		}
	}
	if len(found) != 1 {
		return Card{}, false
	}
	return found[0], true
}

// EngineInstallation is the installation that holds the running engine
// (<installation>/bin/metasystem): the seat a record-bound writer's own card
// names, the armed checkout the host registry records.
func EngineInstallation() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	installation := filepath.Join(filepath.Dir(executable), "..")
	if resolved, err := filepath.EvalSymlinks(installation); err == nil {
		return resolved, nil
	}
	return filepath.Clean(installation), nil
}

// SeatInstallation is the installation a seat's own cards name: a writer
// that moves a goal to another seat of this host (a handover) names that
// seat as the seat names itself. False when the seat has written no card.
func SeatInstallation(home, machine string) (string, bool) {
	if !SafeName(machine) {
		return "", false
	}
	entries, err := os.ReadDir(filepath.Join(Dir(home), machine))
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		goal, ok := strings.CutSuffix(entry.Name(), ".json")
		if !ok || !SafeName(goal) {
			continue
		}
		if card, ok := readCardFile(filepath.Join(Dir(home), machine, entry.Name())); ok && card.Seat.Machine == machine && card.Seat.Installation != "" {
			return card.Seat.Installation, true
		}
	}
	return "", false
}

// notADirectory refuses a board path something else stands at.
func notADirectory(path string) error {
	return coded("BOARD_PATH_NOT_A_DIRECTORY", "path="+path,
		fmt.Errorf("no card was written: %s is a file or symlink, not a directory; move it aside", path))
}

// boardUnreadable is the refusal of a board that cannot be read.
func boardUnreadable(err error) error {
	return coded("BOARD_UNREADABLE", "", fmt.Errorf("the host's message board cannot be read: %w", err))
}

// codedError is a board refusal: Error is the plain reason; the register code
// and facts are its detail (it satisfies refusal.Coder, which the board may
// not import).
type codedError struct {
	code, facts string
	reason      error
}

func (e *codedError) Error() string       { return e.reason.Error() }
func (e *codedError) Unwrap() error       { return e.reason }
func (e *codedError) RefusalCode() string { return e.code }
func (e *codedError) RefusalDetail() string {
	return strings.TrimSpace(e.code+" "+e.facts) + ": " + e.reason.Error()
}

func coded(code, facts string, reason error) error {
	return &codedError{code: code, facts: facts, reason: reason}
}
