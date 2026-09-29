package evidence

// The bound's exclusions (design engine-owns-disk-lifetimes Part B 3.12,
// "The ledger observation" and "Exclusions, regardless of the bound"):
// before any exclusion is judged in a segment, one current observation of
// the segment's goal ledger is taken (fetched first; a person's preview
// reads the accepted tree as it stands and fetches nothing), and its ledger
// identity must be the one the segment index recorded. Then per item: a
// goal still open holds it; a receipt line no retro covered that names it
// holds it; a record that cites it by path holds it. Any Unknown keeps the
// item; a segment-wide Unknown holds every item of the segment.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Goal states a ledger view answers.
const (
	GoalOpen      = "open"
	GoalDone      = "done"
	GoalAbandoned = "abandoned"
)

// LedgerView is one observation of a checkout's goal ledger.
type LedgerView struct {
	Tip      string
	Identity string
	// Committed is the accepted commit's time: how old a view is.
	Committed time.Time
	// Local is goal.sync-remote=local: the view is the union of every
	// armed clone of the identity.
	Local  bool
	States map[string]string
}

// Observer takes one observation of the ledger of the checkout whose
// installation is given; fetch false reads the accepted tree as it stands.
type Observer func(ctx context.Context, installation string, fetch bool) (LedgerView, error)

// Citer answers the citation clause for one item: the files that cite it,
// an Unknown (a file that cannot be read), or pending (too many changed
// files to scan within the budget).
type Citer interface {
	Cited(ctx context.Context, segment Segment, item Item) (files []string, unknown, pending string)
}

// Exclusions judges one pass's items; its observations and receipt
// ledgers are taken once per checkout and reused for every item.
type Exclusions struct {
	Observe Observer
	// Fetch is false for a person's preview (R15): nothing is fetched and
	// no ref advances.
	Fetch bool
	// Peers are the armed checkouts of the host, for the local-mode union.
	Peers []Context
	// Unreadable are armed checkouts whose facts could not be read: in
	// local mode their ledgers are part of the union, so an identity they
	// might carry is Unknown (Round B2, F-10).
	Unreadable []string
	// Tip reads a checkout's accepted ledger tip cheaply (no fetch); every
	// item's judgement checks it and re-projects only when it moved since
	// the last judgement (Round B2, F-7). nil re-observes every item.
	Tip       func(ctx context.Context, installation string) (string, error)
	Citations Citer
	views     map[string]observation
	ledgers   map[string]*receiptLedger
}

type observation struct {
	view LedgerView
	err  string
}

// Judge is the exclusions' answer for one item.
func (e *Exclusions) Judge(ctx context.Context, segment Segment, item Item) Judgement {
	if segment.Context == nil {
		return Judgement{SegmentUnknown: "the segment has no armed context checkout"}
	}
	view, unknown := e.observe(ctx, segment)
	judgement := Judgement{LedgerTip: view.Tip, LedgerIdentity: view.Identity}
	if unknown != "" {
		judgement.SegmentUnknown = unknown
		return judgement
	}
	e.goalClause(segment, item, view, &judgement)
	ledger := e.receipts(segment.Context.Installation)
	if ledger.err != nil {
		judgement.SegmentUnknown = "the receipt ledger cannot be read: " + ledger.err.Error()
		return judgement
	}
	if count := ledger.names(item); count > 0 {
		judgement.Uncovered = count
		judgement.Held = append(judgement.Held, fmt.Sprintf("named by %d receipt(s) no retro covered", count))
	}
	judgement.Commit = ledger.recheck
	if e.Citations == nil {
		if judgement.Unknown == "" {
			judgement.Unknown = "the citation index is not available"
		}
		return judgement
	}
	files, unknownCitation, pending := e.Citations.Cited(ctx, segment, item)
	switch {
	case unknownCitation != "":
		judgement.SegmentUnknown = unknownCitation
	case pending != "":
		if judgement.Unknown == "" {
			judgement.Unknown = pending
		}
	case len(files) > 0:
		judgement.Citations = len(files)
		judgement.Held = append(judgement.Held, "cited by "+files[0]+moreThan(len(files)))
	}
	return judgement
}

func moreThan(count int) string {
	if count > 1 {
		return fmt.Sprintf(" and %d more", count-1)
	}
	return ""
}

// observe takes the segment's one observation (cached per installation for
// the pass) and checks its identity against the segment's index.
func (e *Exclusions) observe(ctx context.Context, segment Segment) (LedgerView, string) {
	if e.views == nil {
		e.views = map[string]observation{}
	}
	installation := segment.Context.Installation
	observed, cached := e.views[installation]
	switch {
	case !cached:
		observed = e.take(ctx, installation, e.Fetch)
	case e.Tip == nil:
		observed = e.take(ctx, installation, e.Fetch)
	case observed.view.Local:
		// Local mode: the union spans every armed clone of the identity,
		// and any of them may have moved; each is read afresh from its
		// accepted ledger (Round B2-2, R6).
		observed = e.take(ctx, installation, false)
	default:
		// Each item is judged at its own critical section: the accepted
		// tip is read again, and a moved tip is projected afresh.
		if tip, err := e.Tip(ctx, installation); err != nil || tip != observed.view.Tip {
			observed = e.take(ctx, installation, false)
		}
	}
	e.views[installation] = observed
	if observed.err != "" {
		return observed.view, "ledger not observed: " + observed.err
	}
	view := observed.view
	switch {
	case view.Identity == "":
		return view, "ledger not observed: the accepted root record has no ledger identity"
	case segment.LedgerIdentity != "" && view.Identity != segment.LedgerIdentity:
		return view, "ledger identity changed: " + segment.LedgerIdentity + " now " + view.Identity
	case segment.LedgerIdentity == "":
		return view, "the segment index records no ledger identity"
	}
	return view, ""
}

func (e *Exclusions) take(ctx context.Context, installation string, fetch bool) observation {
	if e.Observe == nil {
		return observation{err: "no ledger observer in this engine"}
	}
	view, err := e.Observe(ctx, installation, fetch)
	if err != nil {
		return observation{view: view, err: err.Error()}
	}
	if !view.Local {
		return observation{view: view}
	}
	// Local mode: a goal open in any armed clone of this identity is open;
	// a clone whose identity cannot be read may be one of them.
	if len(e.Unreadable) > 0 {
		return observation{view: view, err: "local mode: the ledger facts of " + strings.Join(e.Unreadable, ", ") + " cannot be read, so the union for this identity is unknown"}
	}
	union := LedgerView{Tip: view.Tip, Identity: view.Identity, Committed: view.Committed, Local: true, States: map[string]string{}}
	for id, state := range view.States {
		union.States[id] = state
	}
	for _, peer := range e.Peers {
		if peer.Installation == installation {
			continue
		}
		// A peer whose identity read failed may be a clone of this one:
		// the whole union is Unknown (Round B2-3, rule 4).
		if peer.Facts.LedgerIdentity == "" {
			return observation{view: view, err: "local mode: the ledger identity of " + peer.Installation + " cannot be read, so the union for this identity is unknown"}
		}
		if peer.Facts.LedgerIdentity != view.Identity {
			continue
		}
		other, err := e.Observe(ctx, peer.Installation, false)
		if err != nil {
			return observation{view: view, err: "the local-mode clone " + peer.Installation + " was not observed: " + err.Error()}
		}
		for id, state := range other.States {
			if state == GoalOpen || union.States[id] == "" {
				union.States[id] = state
			}
		}
	}
	return observation{view: union}
}

// goalClause is clause 1: a chain's goal from its root record, a bundle's
// from OWNER.json (none skips, unknown is Unknown, and its recorded ledger
// identity must be the one observed); an events archive has none.
func (e *Exclusions) goalClause(segment Segment, item Item, view LedgerView, judgement *Judgement) {
	id := item.Goal
	switch item.Kind {
	case diskstore.KindEvents:
		return
	case diskstore.KindBundle:
		switch {
		case id == diskstore.GoalNone:
			return
		case id == "" || id == diskstore.GoalUnknown:
			judgement.Unknown = "the bundle's owner names no known goal"
			return
		case item.OwnerLedger == "":
			judgement.Unknown = "the bundle's owner file records no ledger identity"
			return
		case item.OwnerLedger != view.Identity:
			judgement.Unknown = "the bundle was written under ledger " + item.OwnerLedger + ", not " + view.Identity
			return
		}
	case diskstore.KindChain:
		if id == "" {
			return
		}
	}
	state, known := view.States[id]
	switch {
	case !known:
		judgement.Unknown = "goal " + id + " is not in the accepted ledger"
	case state == GoalOpen:
		judgement.GoalState = state
		judgement.Held = append(judgement.Held, "goal "+id+" open")
	default:
		judgement.GoalState = state
	}
}

// receiptLedger is one checkout's receipt ledger as the pass read it.
type receiptLedger struct {
	path      string
	digest    string
	uncovered []string
	err       error
}

var ledgerLine = regexp.MustCompile(`^[0-9]+\|`)

func (e *Exclusions) receipts(installation string) *receiptLedger {
	if e.ledgers == nil {
		e.ledgers = map[string]*receiptLedger{}
	}
	if ledger, ok := e.ledgers[installation]; ok {
		return ledger
	}
	path, err := ReceiptLedgerPath(installation)
	ledger := &receiptLedger{path: path, err: err}
	e.ledgers[installation] = ledger
	if err != nil {
		return ledger
	}
	data, err := os.ReadFile(ledger.path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return ledger
	case err != nil:
		ledger.err = err
		return ledger
	}
	ledger.digest = ledgerDigest(data)
	for _, line := range strings.Split(string(data), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !ledgerLine.MatchString(trimmed) {
			ledger.err = fmt.Errorf("%s is malformed: a line does not start with its epoch", ledger.path)
			return ledger
		}
	}
	ledger.uncovered = receipt.CoverageOf(string(data)).Lines
	return ledger
}

// ReceiptLedgerPath is an installation's receipt ledger under its state
// root. A state root that cannot be resolved is an error: the clause is
// Unknown, never judged against a guessed ledger (Round B2, F-11).
func ReceiptLedgerPath(installation string) (string, error) {
	root, err := stateroot.RootForInstallation(installation)
	if err != nil {
		return "", fmt.Errorf("the state root of %s cannot be resolved: %w", installation, err)
	}
	return filepath.Join(root, "memory", "receipts.log"), nil
}

func ledgerDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// names counts the uncovered lines that name the item: a chain by its root
// or a member job id in delegate=; a bundle by its attempt id.
func (l *receiptLedger) names(item Item) int {
	count := 0
	for _, line := range l.uncovered {
		switch item.Kind {
		case diskstore.KindChain:
			if delegateNames(line, item) {
				count++
			}
		case diskstore.KindBundle:
			if item.Attempt != "" && item.Attempt != diskstore.AttemptStandalone && item.Attempt != diskstore.GoalUnknown && strings.Contains(line, item.Attempt) {
				count++
			}
		}
	}
	return count
}

func delegateNames(line string, item Item) bool {
	members := map[string]bool{item.Name: true}
	for _, job := range item.Jobs {
		members[job] = true
	}
	for _, field := range strings.Split(line, "|") {
		list, ok := strings.CutPrefix(field, "delegate=")
		if !ok {
			continue
		}
		for _, entry := range strings.Split(list, ",") {
			parts := strings.Split(strings.TrimSpace(entry), ":")
			if members[parts[len(parts)-1]] {
				return true
			}
		}
	}
	return false
}

// recheck is the commit point's check (DL4E-07): the ledger re-hashed
// immediately before the receipt append; a change under the judgement is
// Unknown for the item, rolled back and reported.
func (l *receiptLedger) recheck() error {
	data, err := os.ReadFile(l.path)
	now := ""
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return fmt.Errorf("the receipt ledger cannot be re-read at the commit point: %w", err)
	default:
		now = ledgerDigest(data)
	}
	if now != l.digest {
		return errors.New("receipt ledger changed during the judgement")
	}
	return nil
}

// GoalLedgerObserver observes a checkout's ledger through the goal owner:
// goal.Project with fetchFirst, under the context (a projection that
// outlives it is abandoned, never waited for).
func GoalLedgerObserver(now func() time.Time) Observer {
	return func(ctx context.Context, installation string, fetch bool) (LedgerView, error) {
		type answer struct {
			view LedgerView
			err  error
		}
		done := make(chan answer, 1)
		go func() {
			endpoint, err := goal.ResolveEndpoint(installation)
			if err != nil {
				done <- answer{err: err}
				return
			}
			projection, err := goal.Project(endpoint, fetch, now())
			if err != nil {
				done <- answer{err: err}
				return
			}
			view := LedgerView{Tip: projection.Tip, Local: endpoint.LocalMode(), States: map[string]string{}}
			if tree := projection.Tree; tree != nil {
				if tree.Root != nil {
					view.Identity = tree.Root.Identity
				}
				for id := range tree.Live {
					view.States[id] = GoalOpen
				}
				for id := range tree.Done {
					view.States[id] = GoalDone
				}
				for id := range tree.Abandoned {
					view.States[id] = GoalAbandoned
				}
			}
			done <- answer{view: view}
		}()
		select {
		case result := <-done:
			if result.err == nil {
				result.view.Committed = commitTime(ctx, installation, result.view.Tip)
			}
			return result.view, result.err
		case <-ctx.Done():
			return LedgerView{}, fmt.Errorf("the ledger observation did not finish within the pass budget: %w", ctx.Err())
		}
	}
}

// commitTime is a commit's committer time, zero when it cannot be read.
func commitTime(ctx context.Context, root, commit string) time.Time {
	if commit == "" {
		return time.Time{}
	}
	out, err := exec.CommandContext(ctx, "git", "-C", root, "show", "-s", "--format=%cI", commit).Output()
	if err != nil {
		return time.Time{}
	}
	at, _ := time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
	return at
}

// AcceptedTipReader reads a checkout's accepted goal ledger tip under the
// context: one rev-parse, no fetch.
func AcceptedTipReader() func(ctx context.Context, installation string) (string, error) {
	return func(ctx context.Context, installation string) (string, error) {
		out, err := exec.CommandContext(ctx, "git", "-C", installation, "rev-parse", "--verify", "--quiet", goal.AcceptedRef).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}
}
