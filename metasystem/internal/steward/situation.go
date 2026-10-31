package steward

// The situation is one read of what this seat's steward needs: the lane and
// its current hold, the ledger and the program's order, and the claim quota
// read live. A reader that fails makes its part unavailable and the rest
// still read. Nothing here writes.

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginepin"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
)

// Situation is one read's parts. GoalFacts are the seat brief's facts of
// each goal this seat holds. Unavailable names each part whose reader failed
// (lane, hold, ledger, waiting, quota, facts, asks, host, engine, launches,
// designs, questions, authority, alerts) with the reason.
type Situation struct {
	At      time.Time
	Machine string
	Lane    plain.Status
	// Hold is the lane's current proof-beside-builds hold; nil is none.
	Hold        *plain.ContentionHold
	Ledger      goal.Projection
	Next        goal.NextVerdict
	Order       []string
	ClaimLines  []string
	Waiting     []string
	Quota       []ClaimQuotaBlock
	GoalFacts   map[string]string
	Asks        []SituationAsk
	Unavailable map[string]string

	// LaneInstall is the registered lane's installation; empty is none.
	LaneInstall    string
	Load           hostload.Sample
	Processes      []HostProcess
	Executions     map[string]enginepin.Execution
	Holders        []SeatHolder
	Launches       []launch.Record
	Designs        []SituationDesign
	DesignProblems []project.Problem
	// Questions and Alerts are keyed here and, when it is another
	// installation, lane.
	Questions map[string][]channel.Question
	Grants    []goal.PowerOfAttorneyEntry
	Answered  []string
	Rulings   []rulings.Row
	Alerts    map[string][]AlertEpisode
	// RedLine is how goal next says this seat's first owned red.
	RedLine string
}

// HostProcess is one test process on the host: its kind by argv, the
// registered checkout its working directory lies in (else elsewhere) and the
// workspace (owner/name) or worktree under that checkout.
type HostProcess struct {
	Pid                       int64
	Kind, Checkout, Workspace string
	Started                   time.Time
}

// SeatHolder is who holds an armed checkout's lease; Unavailable is this
// seat's read error.
type SeatHolder struct {
	Checkout, MainId, SessionId, Lineage string
	Pid                                  int64
	Unavailable                          string
}

// SituationDesign is one design record with its revision and critique head
// lines and the design-critic chains of its goals.
type SituationDesign struct {
	Record             project.Record
	Revision, Critique string
	Reviews            []dispatch.DesignCritiqueChain
}

// ClaimQuotaBlock is one holding goal, the ready goals its claim holds back
// in the program's order, and the step that frees the claim.
type ClaimQuotaBlock struct {
	Held    string
	Ready   []string
	Command string
}

// SituationAsk is one thread of this seat or its goals: its state at the
// read, when it closed, and whether this machine was offered its ask.
type SituationAsk struct {
	Thread    board.Thread
	State     string
	Since     time.Time
	Delivered bool
}

// SituationSources are the reads the situation makes outside the steward. A
// nil LaneStatus, ClaimLaneState or Waiting leaves its part unavailable.
type SituationSources struct {
	Now            func() time.Time
	Machine        func(string) (string, error)
	LaneHome       func() (string, error)
	LaneStatus     func(home string, now time.Time) plain.Status
	HostWork       func() (int, int, error)
	Project        func(goal.Endpoint, bool, time.Time) (goal.Projection, error)
	ClaimLaneState func(root, goalID, main string) (string, error)
	// Waiting reads the durable waiting lines of the checkout at root.
	Waiting func(root string) ([]string, error)
	// Units reads a goal's units over the read's one projection and clock;
	// nil leaves the seat brief's own reader, which says it is unavailable.
	Units     func(root, goalID string, projection goal.Projection, now time.Time) ([]UnitStage, error)
	BoardHome func() (string, error)
	// The host's reads; a nil one reads the kernel and the host registry.
	Load       func(time.Time) hostload.Sample
	Processes  identity.ProcessTable
	Argv       func(int64) ([]string, bool)
	Cwd        func(int64) (string, bool)
	Prober     identity.Prober
	Registry   func() (string, error)
	Executions func(root string) map[string]enginepin.Execution
	// Holder reads an armed checkout's lease and Launches the launches as
	// the launch verbs see them; nil leaves the part unavailable.
	Holder   func(checkout string) (SeatHolder, error)
	Launches func() ([]launch.Record, error)
	// RedLine words an owned red as goal next does; nil leaves it unworded.
	RedLine func(goal.TrunkRedEntry, time.Time) string
}

// ReadSituation reads every part once, in a fixed order, at one clock.
func ReadSituation(root string, src SituationSources) Situation {
	if src.Machine == nil {
		src.Machine = goal.ResolveMachine
	}
	if src.LaneHome == nil {
		src.LaneHome = board.Home
	}
	if src.BoardHome == nil {
		src.BoardHome = board.Home
	}
	src = withHostReaders(src)
	now := src.Now()
	s := Situation{At: now, Unavailable: map[string]string{}}
	machine, machineErr := src.Machine(root)
	s.Machine = machine
	s.readLane(src, now)
	ledger := newHealthLedger(root, now)
	ledger.laneState = src.ClaimLaneState
	if src.Project != nil {
		ledger.project = src.Project
	}
	if machineErr == nil {
		s.readLedger(root, machine, ledger, now)
	}
	if len(s.Next.TrunkRedOwned) > 0 && src.RedLine != nil {
		s.RedLine = src.RedLine(s.Next.TrunkRedOwned[0], now)
	}
	err := errors.New("no reader")
	if src.Waiting != nil {
		s.Waiting, err = src.Waiting(root)
	}
	if err != nil {
		s.Unavailable["waiting"] = err.Error()
	}
	if machineErr != nil {
		s.Unavailable["ledger"] = "this machine cannot be named: " + machineErr.Error()
		for _, part := range []string{"quota", "facts", "asks"} {
			s.Unavailable[part] = s.Unavailable["ledger"]
		}
	} else {
		s.readQuota(root, ledger)
		s.readFacts(root, src, ledger, now)
		s.readAsks(src, now)
	}
	s.readHost(root, src, now)
	err = errors.New("no reader")
	if src.Launches != nil {
		s.Launches, err = src.Launches()
	}
	if err != nil {
		s.Unavailable["launches"] = err.Error()
	}
	s.readDesigns(root)
	s.readQuestions(root)
	s.readAuthority(root, now)
	s.readAlerts(root)
	return s
}

// withHostReaders gives each nil host read the kernel's or the host
// registry's own reader.
func withHostReaders(src SituationSources) SituationSources {
	if src.Load == nil {
		src.Load = hostload.Read
	}
	if src.Processes == nil {
		src.Processes = identity.KernelProcessTable{}
	}
	if src.Argv == nil {
		src.Argv = identity.KernelProber{}.ReadArgv
	}
	if src.Cwd == nil {
		src.Cwd = identity.ProcessCwd
	}
	if src.Prober == nil {
		src.Prober = identity.KernelProber{}
	}
	if src.Registry == nil {
		src.Registry = registry.DefaultPath
	}
	if src.Executions == nil {
		src.Executions = func(root string) map[string]enginepin.Execution { return ReadExecutions(root, nil) }
	}
	return src
}

// readHost reads the load, the engine roles, then from one registry read
// each test process by checkout and each armed seat's holder. A failed
// registry read leaves the load and the engine roles.
func (s *Situation) readHost(root string, src SituationSources, now time.Time) {
	s.Load = src.Load(now)
	s.Executions = src.Executions(root)
	path, err := src.Registry()
	var checkouts []registry.HostCheckout
	if err == nil {
		checkouts, err = registry.HostCheckouts(path)
	}
	if err != nil {
		s.Unavailable["host"] = "the host registry: " + err.Error()
		s.Unavailable["engine"] = s.Unavailable["host"]
		return
	}
	s.readProcesses(src, checkouts)
	if src.Holder == nil {
		s.Unavailable["engine"] = "no reader"
		return
	}
	for _, checkout := range checkouts {
		if !checkout.Armed {
			continue
		}
		holder, err := src.Holder(checkout.Path)
		if holder.Checkout = checkout.Path; err != nil {
			holder.Unavailable = err.Error()
		}
		s.Holders = append(s.Holders, holder)
	}
}

// readProcesses lists each test process by its argv, and only for those
// reads the start and the working directory: the longest registered
// checkout holding it, else elsewhere.
func (s *Situation) readProcesses(src SituationSources, checkouts []registry.HostCheckout) {
	pids, err := src.Processes.Pids()
	if err != nil {
		s.Unavailable["host"] = "the process table: " + err.Error()
	}
	for _, pid := range pids {
		argv, ok := src.Argv(pid)
		process := HostProcess{Pid: pid, Kind: testProcessKind(argv, ok), Checkout: "elsewhere"}
		if process.Kind == "" {
			continue
		}
		if exact, _, err := src.Prober.Probe(pid); err == nil {
			process.Started = exact.StartedAt
		}
		cwd, ok := src.Cwd(pid)
		for _, checkout := range checkouts {
			inside := cwd == checkout.Path || strings.HasPrefix(cwd, checkout.Path+string(filepath.Separator))
			if ok && inside && (process.Checkout == "elsewhere" || len(checkout.Path) > len(process.Checkout)) {
				process.Checkout, process.Workspace = checkout.Path, workspaceUnder(checkout.Path, cwd)
			}
		}
		s.Processes = append(s.Processes, process)
	}
	slices.SortFunc(s.Processes, func(a, b HostProcess) int {
		return cmp.Or(strings.Compare(a.Checkout, b.Checkout), cmp.Compare(a.Pid, b.Pid))
	})
}

// testProcessKind names a go test, a test binary or a go-build process by
// its argv; anything else is no test process.
func testProcessKind(argv []string, ok bool) string {
	if !ok || len(argv) == 0 {
		return ""
	}
	switch base := filepath.Base(argv[0]); {
	case base == "go" && len(argv) > 1 && argv[1] == "test":
		return "go test"
	case strings.HasSuffix(base, ".test"):
		return "test binary"
	case strings.Contains(argv[0], "/go-build"):
		return "go-build"
	}
	return ""
}

// workspaceUnder is the first workspace (owner/name) or worktree (name) on
// the way from checkout down to cwd; empty is none.
func workspaceUnder(checkout, cwd string) string {
	rel, err := filepath.Rel(checkout, cwd)
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i := 0; err == nil && i+3 < len(parts); i++ {
		if parts[i] != "artifacts" || parts[i+1] != "agents" {
			continue
		}
		if parts[i+2] == "workspaces" && i+4 < len(parts) {
			return parts[i+3] + "/" + parts[i+4]
		}
		if parts[i+2] == "worktrees" {
			return parts[i+3]
		}
	}
	return ""
}

// readDesigns lists every design record with its revision, critique line
// and review chains. A design home's problems make the part incomplete;
// the readable designs stay.
func (s *Situation) readDesigns(root string) {
	roots, err := project.ResolveRoots(root)
	var read *project.Project
	if err == nil {
		read, err = project.Read(roots)
	}
	if err != nil {
		s.Unavailable["designs"] = err.Error()
		return
	}
	var problems []string
	for _, problem := range read.Problems {
		if home, ok := project.HomeFor(roots, problem.Path); ok && home.Kind == project.KindDesign {
			s.DesignProblems = append(s.DesignProblems, problem)
		}
	}
	if len(s.DesignProblems) > 0 {
		problems = append(problems, fmt.Sprintf("incomplete: %d problem(s); first: %s", len(s.DesignProblems), s.DesignProblems[0].String()))
	}
	var chainErr error
	for _, record := range read.Records {
		if record.Kind != project.KindDesign {
			continue
		}
		design := SituationDesign{Record: record, Revision: headValue(record, "Revision"), Critique: headValue(record, "Critique")}
		goals := record.Goals
		if len(goals) == 0 {
			goals = []string{""}
		}
		for _, id := range goals {
			chains, err := dispatch.ReadDesignCritiqueChains(root, id, filepath.Join(roots.Checkout, record.Path))
			design.Reviews, chainErr = append(design.Reviews, chains...), cmp.Or(chainErr, err)
		}
		s.Designs = append(s.Designs, design)
	}
	if chainErr != nil {
		problems = append(problems, "review chains: "+chainErr.Error())
	}
	if len(problems) > 0 {
		s.Unavailable["designs"] = strings.Join(problems, "; ")
	}
}

// headValue is a record head key's value; empty when the head has none.
func headValue(record project.Record, key string) string {
	for _, field := range record.Head {
		if strings.EqualFold(field.Key, key) {
			return field.Value
		}
	}
	return ""
}

// stores are here and, when it is another installation, the lane's.
func (s *Situation) stores(here string) [][2]string {
	stores := [][2]string{{"here", here}}
	if s.LaneInstall != "" && realpath.Resolve(s.LaneInstall) != realpath.Resolve(here) {
		stores = append(stores, [2]string{"lane", s.LaneInstall})
	}
	return stores
}

// readQuestions reads every question, open or ended, here and in the lane.
func (s *Situation) readQuestions(root string) {
	here, err := goal.ResolveStateRoot(root)
	if err != nil {
		s.Unavailable["questions"] = err.Error()
		return
	}
	s.Questions = map[string][]channel.Question{}
	var unreadable []string
	for _, store := range s.stores(here) {
		questions, bad := channel.WalkQuestions(store[1])
		s.Questions[store[0]], unreadable = questions, append(unreadable, bad...)
	}
	if len(unreadable) > 0 {
		s.Unavailable["questions"] = "unreadable: " + strings.Join(unreadable, "; ")
	}
}

// readAuthority keeps the grants live at the read from the read's one
// ledger, the attorney log's last 20 lines and the last 10 rulings.
func (s *Situation) readAuthority(root string, now time.Time) {
	var problems []string
	if reason := s.Unavailable["ledger"]; reason != "" {
		problems = append(problems, "grants: "+reason)
	} else if s.Ledger.Tree.Root != nil {
		for _, entry := range s.Ledger.Tree.Root.PowerOfAttorney {
			if live, _ := entry.LiveAt(now); live {
				s.Grants = append(s.Grants, entry)
			}
		}
	}
	data, err := os.ReadFile(humanauthority.AttorneyLogPath(root))
	if text := strings.TrimRight(string(data), "\n"); err == nil && text != "" {
		lines := strings.Split(text, "\n")
		s.Answered = lines[max(0, len(lines)-20):]
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		problems = append(problems, "the attorney log: "+err.Error())
	}
	register, err := rulings.Read(root)
	if err != nil {
		problems = append(problems, "the rulings: "+err.Error())
	}
	s.Rulings = register.Rows[max(0, len(register.Rows)-10):]
	if len(problems) > 0 {
		s.Unavailable["authority"] = strings.Join(problems, "; ")
	}
}

// readAlerts keeps the uncleared episodes here and in the lane. A store
// with no alerts folder has none and is not locked, so no folder is made.
func (s *Situation) readAlerts(root string) {
	s.Alerts = map[string][]AlertEpisode{}
	var problems []string
	for _, store := range s.stores(root) {
		if _, err := os.Stat(alertDir(store[1])); errors.Is(err, fs.ErrNotExist) {
			continue
		}
		episodes, err := AlertEpisodes(store[1])
		if err != nil {
			problems = append(problems, store[0]+": "+err.Error())
		}
		for _, episode := range episodes {
			if !episode.Cleared {
				s.Alerts[store[0]] = append(s.Alerts[store[0]], episode)
			}
		}
	}
	if len(problems) > 0 {
		s.Unavailable["alerts"] = strings.Join(problems, "; ")
	}
}

// readLane reads the lane's status and its current hold at the read's clock
// and through the given observation, so the two agree. Unknown is held.
func (s *Situation) readLane(src SituationSources, now time.Time) {
	home, err := src.LaneHome()
	record, registered := lane.Record{}, false
	if err == nil {
		record, registered, err = lane.Read(home)
	}
	if err == nil && !registered {
		err = errors.New("no landing lane is registered on this host")
	}
	if err != nil {
		s.Unavailable["lane"], s.Unavailable["hold"] = err.Error(), err.Error()
		return
	}
	s.LaneInstall = record.Install
	if src.LaneStatus == nil {
		s.Unavailable["lane"] = "no reader"
	} else {
		s.Lane = src.LaneStatus(home, now)
	}
	// The hold reads an unobservable host as held; its reason is kept here.
	seams, observed := plain.ProveSeams{Now: func() time.Time { return now }}, error(nil)
	if src.HostWork != nil {
		seams.HostWork = func() (int, int, error) { b, c, err := src.HostWork(); observed = err; return b, c, err }
	}
	hold, held, err := plain.CurrentHold(record.Install, seams)
	if held || err != nil {
		s.Hold = &hold
	}
	if err = errors.Join(err, observed); err != nil && s.Hold != nil {
		s.Unavailable["hold"] = "unknown: " + err.Error()
	}
}

// readLedger reads the frontier, the order and the claim lines from the
// ledger's one projection.
func (s *Situation) readLedger(root, machine string, ledger *healthLedger, now time.Time) {
	err := ledger.read().endpointErr
	if err == nil {
		err = ledger.projectionErr
	}
	if err == nil && !ledger.newWorld {
		err = errors.New("the bootstrap ledger has no claims")
	}
	if err == nil {
		s.Ledger = ledger.projection
		s.Next, err = goal.Next(ledger.projection, machine)
	}
	if err != nil {
		s.Unavailable["ledger"] = err.Error()
		return
	}
	live := ledger.projection.Tree.Live
	s.Order = goal.OrderedOpenGoalIDs(live)
	files := func(ids []string) (out []*goal.GoalFile) {
		for _, id := range ids {
			out = append(out, live[id])
		}
		return out
	}
	s.ClaimLines = append(goal.FencedClaimLines(files(s.Next.Fenced)), goal.LandingClaimLines(files(s.Next.Landing), now, dispatch.LandingOverdue(root))...)
}

// seatGoals are the goals this seat holds: claimed, landing and fenced.
func (s *Situation) seatGoals() []string {
	return slices.Concat(s.Next.Claimed, s.Next.Landing, s.Next.Fenced)
}

// readFacts writes each seat goal's facts as the seat brief writes them,
// over the read's one projection and clock.
func (s *Situation) readFacts(root string, src SituationSources, ledger *healthLedger, now time.Time) {
	if reason := s.Unavailable["ledger"]; reason != "" {
		s.Unavailable["facts"] = reason
		return
	}
	d := defaultSeatDependencies(nil)
	d.Now = func() time.Time { return now }
	d.Project = func(string, time.Time) (goal.Projection, error) { return ledger.projection, nil }
	d.Machine = func(string) (string, error) { return s.Machine, nil }
	if src.Units != nil {
		d.Units = func(root, id string) ([]UnitStage, error) { return src.Units(root, id, ledger.projection, now) }
	}
	ids := s.seatGoals()
	tips, err := d.Tips(root, ids)
	if err != nil {
		s.Unavailable["facts"] = "goal tips: " + err.Error()
	}
	s.GoalFacts = map[string]string{}
	for _, id := range ids {
		s.GoalFacts[id] = seatFacts(root, SeatSelection{Goal: id}, s.Machine, tips, d)
	}
}

// readAsks keeps every thread to or from this seat or to its goals, in
// every state, with this machine's own delivery marker. It marks nothing.
func (s *Situation) readAsks(src SituationSources, now time.Time) {
	home, err := src.BoardHome()
	var threads []board.Thread
	if err == nil {
		threads, err = board.Threads(home)
	}
	if err != nil {
		s.Unavailable["asks"] = err.Error()
		return
	}
	goals := s.seatGoals()
	for _, thread := range threads {
		if !slices.ContainsFunc(thread.Messages, func(m board.Message) bool {
			return m.To.Machine == s.Machine || m.From.Machine == s.Machine || slices.Contains(goals, m.To.Goal)
		}) {
			continue
		}
		state, since := thread.State(now)
		s.Asks = append(s.Asks, SituationAsk{Thread: thread, State: state, Since: since, Delivered: thread.Root != nil && thread.Root.Offered(s.Machine)})
	}
}

// readQuota asks the claim quota on the read's ledger through the live lane
// reader, one block per holding goal. Without a reader it reads nothing:
// never the tick's saved verdict.
func (s *Situation) readQuota(root string, ledger *healthLedger) {
	if ledger.laneState == nil {
		s.Unavailable["quota"] = "no lane reader"
		return
	}
	verdict := checkClaimQuotaWith(root, root, ledger)
	if verdict.Status == HealthUnknown {
		s.Unavailable["quota"] = verdict.Reason
	}
	for _, fact := range verdict.RemedyFacts {
		if verdict.Status != HealthDead || fact.Cause != CauseClaimQuota {
			continue
		}
		at := slices.IndexFunc(s.Quota, func(block ClaimQuotaBlock) bool { return block.Held == fact.Goal })
		if at < 0 {
			at, s.Quota = len(s.Quota), append(s.Quota, ClaimQuotaBlock{Held: fact.Goal, Command: fact.Command})
		}
		s.Quota[at].Ready = append(s.Quota[at].Ready, fact.Ready)
	}
	for _, block := range s.Quota {
		slices.SortStableFunc(block.Ready, func(a, b string) int { return slices.Index(s.Order, a) - slices.Index(s.Order, b) })
	}
}
