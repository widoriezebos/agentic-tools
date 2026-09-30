package launch

// The preflight: what is judged before any step runs and before the host
// lock is taken.
//
// It is separate from the sequencer because it answers a different question.
// The sequencer asks whether each step's precondition holds and redoes what
// does not; the preflight asks whether this launch should begin at all, and
// every one of its refusals is a fact about the host rather than about a
// step: a nickname that cannot publish, a nickname somebody already carries,
// a destination that is there, a destination inside another checkout, a seat
// with no evidence root of its own to compare the new machine's against.
//
// A resume is exempted from exactly two of them, and only for what the record
// says this launch itself created: the destination it made, and the nickname
// it set. Everything else is judged again, because a launch that was
// interrupted does not make the world it was launched into stand still.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
)

// Request is one launch as its caller asks for it.
type Request struct {
	// Machine is the nickname the new machine carries in its own git
	// configuration and publishes presence under.
	Machine string
	// From is the launching checkout, whose objects the clone comes from.
	From string
	// Destination is the absolute path of the clone on this host.
	Destination string
	// Word and ReviewBy are the temporary pair a human at a terminal may
	// give `seat launch`, which travel to the arming verb's argument list
	// and nowhere else. The word is never written into the record, the
	// capture or a log. The interface never sends them: a browser launch is
	// enrolled from the signed-in session's verdict on its record (g1-s72).
	Word     string
	ReviewBy string
	// Resume names the launch this run continues, or "" for a fresh one.
	Resume string
}

// Resuming reports whether this request continues an earlier launch.
func (r Request) Resuming() bool { return r.Resume != "" }

// Admit judges a request against the record it names, before anything is
// written or run.
//
// A record a signed-in session enrolled is authoritative for its machine and
// its destination, on a fresh invocation exactly as on a resume (g1-s72
// S72-01): the clone's arm binds the verdict to the record's destination, so
// a stale record path handed a new machine and destination would otherwise
// enroll another machine under the session that launched the first. A flag
// left empty takes the record's value; one that differs is refused. The pair
// beside such a record is refused too: the record already says whose machine
// this is. A record with no enrollment keeps the verb's own rules.
func Admit(request Request, record Record) error {
	if !record.SessionEnrolled() {
		return nil
	}
	if request.Word != "" || request.ReviewBy != "" {
		return refuse(CodeWordInvalid,
			"%s already approved launch %s in the browser, so it takes no temporary word",
			record.Enrollment.Human, record.Launch).run(resumeCommand(record.Machine, record.Launch))
	}
	if request.Machine != "" && request.Machine != record.Machine {
		return refuse(CodeRecordConflict,
			"launch %s starts %s, not %s: %s approved it for that machine",
			record.Launch, record.Machine, request.Machine, record.Enrollment.Human).run(resumeCommand(record.Machine, record.Launch))
	}
	if request.Destination != "" && filepath.Clean(request.Destination) != filepath.Clean(record.Destination) {
		return refuse(CodeRecordConflict,
			"launch %s clones into %s, not %s: its approval names that place",
			record.Launch, record.Destination, request.Destination).run(resumeCommand(record.Machine, record.Launch))
	}
	return nil
}

// Facts are the world the preflight judges against, gathered by the caller so
// that this function opens no repository and reads no ledger.
type Facts struct {
	// This is the launching checkout's own nickname, or "" where it has
	// none. A machine may not be launched under the nickname of the seat
	// launching it: the two would publish over each other.
	This string
	// Taken is every nickname already spoken for on this fleet: a sibling
	// clone beside this checkout, a presence ref, or a claim at the accepted
	// tip.
	Taken []string
	// Created is what the launch a resume names created, all false for a
	// fresh launch. It is the whole of what a resume is exempted from.
	Created Created
	// EvidenceRoot is this seat's effective evidence root, as the engine's
	// one owner resolves it. The launch compares the new machine's root with
	// it, so a seat whose root cannot be named is refused here, before the
	// clone leaves a directory for a human to delete.
	EvidenceRoot string
}

// Preflight judges one request against the host and refuses by name.
func Preflight(request Request, facts Facts) error {
	if err := nickname(request, facts); err != nil {
		return err
	}
	if err := destination(request, facts); err != nil {
		return err
	}
	return evidenceRootSet(facts)
}

// evidenceRootSet refuses a seat whose evidence root is not a path: unset, or
// still the template's placeholder, which is not absolute.
func evidenceRootSet(facts Facts) error {
	if !filepath.IsAbs(facts.EvidenceRoot) {
		return refuse(CodeEvidenceRootUnsafe,
			"this seat's evidence root is not set (it reads %q), so it cannot give the new machine one", facts.EvidenceRoot).run(evidenceRootCommand)
	}
	return nil
}

// startAt is the launch again with another destination.
func startAt(request Request, destination string) string {
	command := "metasystem machine start " + request.Machine + " --destination " + destination
	if request.Resuming() {
		command += " --resume " + request.Resume
	}
	return command
}

// nickname refuses a name the presence publisher would refuse, this seat's
// own, and one somebody already carries.
func nickname(request Request, facts Facts) error {
	if err := seat.ValidateMachineName(request.Machine); err != nil {
		return refuse(CodeNicknameInvalid, "%s", err.Error()).run("metasystem machine start <another name>")
	}
	if facts.This != "" && request.Machine == facts.This {
		return refuse(CodeNicknameInvalid,
			"%s is this checkout's own name; a new machine needs a name of its own", request.Machine).run("metasystem machine start <another name>")
	}
	// A resume redoes the step that sets the nickname, so the name this
	// launch itself set is not somebody else carrying it.
	if facts.Created.Nickname && request.Resuming() {
		return nil
	}
	for _, held := range facts.Taken {
		if held == request.Machine {
			return refuse(CodeNicknameTaken,
				"%s is already the name of a machine in this fleet", request.Machine).run("metasystem machine start <another name>")
		}
	}
	return nil
}

// destination refuses a path that is not absolute, one that is already there,
// and one that lies inside another checkout.
func destination(request Request, facts Facts) error {
	if !filepath.IsAbs(request.Destination) {
		return refuse(CodeDestinationExists,
			"the destination %q is not an absolute path on this host", request.Destination).run(startAt(request, "<an absolute path>"))
	}
	path := filepath.Clean(request.Destination)
	_, err := os.Lstat(path)
	switch {
	case err == nil && facts.Created.Destination && request.Resuming():
		// The launch being resumed made this directory; it is this launch's
		// to finish rather than somebody else's to be refused for.
	case err == nil:
		return refuse(CodeDestinationExists,
			"%s already exists, and a new machine needs an empty place for its clone", path).run(startAt(request, "<an empty place>"))
	case !os.IsNotExist(err):
		return refuse(CodeDestinationExists, "the destination %s could not be read (%v)", path, err).run(startAt(request, "<another place>"))
	}
	inside, where, err := insideACheckout(path)
	if err != nil {
		return refuse(CodeDestinationInsideACheckout, "the folders above %s could not be read (%v)", path, err).run(startAt(request, "<another place>"))
	}
	if inside {
		return refuse(CodeDestinationInsideACheckout,
			"%s lies inside the checkout at %s; a new machine goes beside the others", path, where).run(startAt(request, "<a place beside "+where+">"))
	}
	return nil
}

// insideACheckout reports whether a path lies beneath a git checkout, and
// which one.
//
// It walks up from the destination's parent rather than from the destination
// itself, because a resumed launch's destination IS a checkout — the one this
// launch cloned — and a check that started there would refuse every resume.
func insideACheckout(path string) (bool, string, error) {
	for directory := filepath.Dir(path); ; {
		_, err := os.Lstat(filepath.Join(directory, ".git"))
		if err == nil {
			return true, directory, nil
		}
		if !os.IsNotExist(err) {
			return false, "", err
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return false, "", nil
		}
		directory = parent
	}
}

// Siblings is every nickname a clone beside this checkout carries, read from
// the directory names this verb itself proposes destinations from.
//
// It is a name-shaped reading and not a git one on purpose: the question is
// which nickname is spoken for on this host, and a directory named
// <repository>-<nickname> is the answer this verb's own naming rule makes.
// The authoritative readings — the presence refs and the accepted tip — are
// the caller's and join this one in Facts.Taken.
func Siblings(parent, repository string) []string {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		nickname, beneath := strings.CutPrefix(entry.Name(), repository+"-")
		if !beneath || nickname == "" {
			continue
		}
		names = append(names, nickname)
	}
	return names
}

// DefaultDestination is where a machine lands unless a human says otherwise:
// beside this checkout, named for the ledger remote's own repository with the
// nickname appended.
//
// The repository's name is the remote's and never a fixed word, so another
// project launching a machine gets its own name rather than this one's.
func DefaultDestination(checkout, repository, nickname string) string {
	return filepath.Join(filepath.Dir(filepath.Clean(checkout)), repository+"-"+nickname)
}

// RepositoryName is the repository a ledger remote URL names, which is the
// last path segment without its .git suffix. An URL nothing can be read from
// answers "", and the caller proposes no destination rather than one named
// after a guess.
func RepositoryName(remoteURL string) string {
	trimmed := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(remoteURL), "/"), ".git")
	if trimmed == "" {
		return ""
	}
	if at := strings.LastIndexAny(trimmed, "/:"); at >= 0 {
		trimmed = trimmed[at+1:]
	}
	return trimmed
}
