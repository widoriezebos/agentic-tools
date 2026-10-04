package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// The alert verbs (design steward-acts-on-behaviour-patterns §1): every
// AlertEpisode the steward keeps (health, spend, seat-idle, pattern) in this
// checkout's store and the host lane's. A person acts on a qualified id,
// <store>/<on-disk id>: lane is the host lane's store, here the checkout
// serving the command; on the lane checkout both are one store, shown as
// lane.

// alertOwners are the alert verbs' seams; the zero value is production.
type alertOwners struct {
	home    func() (string, error)
	now     func() time.Time
	invoker func() (steward.AlertInvoker, error)
}

func (inv *intentInvocation) alerts() alertOwners {
	owners := inv.owners.alerts
	if owners.home == nil {
		owners.home = board.Home
	}
	if owners.now == nil {
		owners.now = time.Now
	}
	if owners.invoker == nil {
		owners.invoker = alertInvoker
	}
	return owners
}

// alertInvoker is what can be observed about this process, which invoked the
// act; it makes no claim about human authority.
func alertInvoker() (steward.AlertInvoker, error) {
	self, state, err := identity.KernelProber{}.Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return steward.AlertInvoker{}, fmt.Errorf("this process cannot read its own identity")
	}
	return steward.AlertInvoker{Pid: self.Pid, PidStartedAt: self.StartedAt.Unix(), PidStartTicks: self.StartTicks, BootID: self.BootID, UID: os.Getuid()}, nil
}

func alertIntentCommands() []intentCommand {
	return []intentCommand{{
		object: "alert", action: "list", audience: "both", summary: "the open alerts of this checkout and the host's landing lane",
		usage:    []string{"metasystem alert list [--all]"},
		details:  []string{"An alert is one finding the steward reported, with its evidence; --verbose adds its work, its evidence and the checkout that keeps it."},
		flags:    []intentFlag{{name: "all", usage: "include cleared alerts"}},
		maxArgs:  0,
		examples: []string{"metasystem alert list"},
		run:      runIntentAlertList,
	}, {
		object: "alert", action: "ack", audience: "both", summary: "mark an alert seen; it stays open",
		usage:    []string{"metasystem alert ack ALERT"},
		details:  []string{"ALERT is the id alert list prints, such as lane/alert-1e4e80ba71c3e452-1. Acknowledging again is fine."},
		maxArgs:  1,
		examples: []string{"metasystem alert ack lane/alert-1e4e80ba71c3e452-1"},
		run:      func(inv *intentInvocation) int { return runIntentAlertAct(inv, "ack") },
	}, {
		object: "alert", action: "clear", audience: "human", summary: "resolve an alert; the same work does not report again until it reads clear",
		usage:    []string{"metasystem alert clear ALERT"},
		details:  []string{"ALERT is the id alert list prints. Clearing again is fine."},
		maxArgs:  1,
		examples: []string{"metasystem alert clear lane/alert-1e4e80ba71c3e452-1"},
		run:      func(inv *intentInvocation) int { return runIntentAlertAct(inv, "clear") },
	}}
}

// alertStore is one store the verbs read: its name and the steward root
// that keeps it.
type alertStore struct{ name, root string }

// alertStores are this checkout's store and the host lane's, the lane's
// first; one store named lane when this checkout is the lane. A lane that
// cannot be resolved leaves this checkout's store alone and says why.
func (inv *intentInvocation) alertStores() ([]alertStore, string, *intentResult) {
	path := inv.cwd
	if inv.input.has("repo") {
		if path = inv.input.text("repo"); !filepath.IsAbs(path) {
			path = filepath.Join(inv.cwd, path)
		}
	}
	here, err := inv.alertRoot(path)
	if err != nil {
		return nil, "", inv.notARepository(path, err)
	}
	stores := []alertStore{{name: "here", root: here}}
	home, err := inv.alerts().home()
	if err != nil {
		return stores, "the host's landing lane could not be read: " + err.Error(), nil
	}
	record, registered, err := lane.Read(home)
	switch {
	case err != nil:
		return stores, "the host's landing lane could not be read: " + err.Error(), nil
	case !registered:
		return stores, "", nil
	}
	laneRoot, err := inv.alertRoot(record.Root)
	if err != nil {
		return stores, "the landing lane's checkout " + record.Root + " could not be read: " + err.Error(), nil
	}
	if realpath.Resolve(laneRoot) == realpath.Resolve(here) {
		return []alertStore{{name: "lane", root: laneRoot}}, "", nil
	}
	return []alertStore{{name: "lane", root: laneRoot}, stores[0]}, "", nil
}

// alertRoot is the steward root of the checkout holding path: its
// installation's state root.
func (inv *intentInvocation) alertRoot(path string) (string, error) {
	layout, err := inv.owners.resolver.ResolveLayout(path)
	if err != nil {
		return "", err
	}
	root, err := inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
	return root.Path(), err
}

type listedAlert struct {
	ID       string               `json:"id"`
	Store    string               `json:"store"`
	Checkout string               `json:"checkout"`
	Episode  steward.AlertEpisode `json:"episode"`
}

func runIntentAlertList(inv *intentInvocation) int {
	stores, laneProblem, problem := inv.alertStores()
	if problem != nil {
		return inv.render(*problem)
	}
	all := inv.input.switched("all")
	listed := []listedAlert{}
	var lines, details []string
	var unreadable []string
	for _, store := range stores {
		episodes, err := steward.AlertEpisodes(store.root)
		if err != nil {
			unreadable = append(unreadable, store.name)
			details = append(details, fmt.Sprintf("the %s store at %s could not be read: %v", store.name, store.root, err))
			continue
		}
		for _, episode := range episodes {
			if episode.Cleared && !all {
				continue
			}
			id := store.name + "/" + episode.EpisodeID
			listed = append(listed, listedAlert{ID: id, Store: store.name, Checkout: store.root, Episode: episode})
			lines = append(lines, "  "+id+"  "+alertSituation(episode), "    "+alertNextLine(id, episode))
			details = append(details, alertDetails(id, store, episode)...)
		}
	}
	if laneProblem != "" {
		details = append(details, laneProblem)
	}
	open := 0
	for _, item := range listed {
		if !item.Episode.Cleared {
			open++
		}
	}
	result := intentResult{Outcome: intentConfirmed, Summary: alertCount(open), text: lines, Details: details,
		Data: map[string]any{"alerts": listed}}
	if all {
		result.Summary += fmt.Sprintf("; %d listed with the cleared ones", len(listed))
	}
	if len(unreadable) > 0 {
		result.Outcome, result.code = intentFailed, 1
		result.Summary = "the " + strings.Join(unreadable, " and ") + " alert store could not be read, so this list is incomplete"
		result.retry = "once the store is readable again"
	}
	return inv.render(result)
}

func alertCount(open int) string {
	switch open {
	case 0:
		return "no open alerts"
	case 1:
		return "1 open alert"
	}
	return fmt.Sprintf("%d open alerts", open)
}

// alertSituation is an episode's line 1: its plain situation, and why it
// neither grows nor clears when it stands still.
func alertSituation(episode steward.AlertEpisode) string {
	line, _, _ := strings.Cut(episode.Message, "\n")
	switch {
	case episode.Cleared:
		line += " (cleared)"
	case episode.Standing == steward.StandingHeld:
		line += " (held by a person; it stands still)"
	case episode.Standing == steward.StandingUnreadable:
		line += " (its signal can't be read now)"
	case episode.Acknowledged:
		line += " (seen)"
	}
	return line
}

// alertNextLine is an episode's line 2: the one command that moves it.
func alertNextLine(id string, episode steward.AlertEpisode) string {
	switch {
	case episode.Cleared:
		return "nothing to do; it is cleared"
	case episode.Acknowledged:
		return "run: metasystem alert clear " + id
	}
	return "run: metasystem alert ack " + id
}

func alertDetails(id string, store alertStore, episode steward.AlertEpisode) []string {
	details := []string{fmt.Sprintf("%s: owner %s, work %s, kept in %s", id, alertOwnerWords(episode.Owner), alertOr(episode.ScopeID, "none"), store.root)}
	for _, item := range episode.Evidence {
		details = append(details, fmt.Sprintf("%s: evidence %s %s %s", id, item.At, item.Record, item.Fact))
	}
	return details
}

func alertOwnerWords(owner string) string { return alertOr(owner, "health") }

func alertOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// runIntentAlertAct acknowledges or clears one alert by its qualified id,
// or a bare id that names one alert in one store. A repeat succeeds.
func runIntentAlertAct(inv *intentInvocation, act string) int {
	if len(inv.input.args) != 1 {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "no alert is named, so nothing was done",
			next: inv.publicArgv("alert", "list"), nextReason: "names the alerts; then repeat with one of them"})
	}
	named := inv.input.args[0]
	stores, _, problem := inv.alertStores()
	if problem != nil {
		return inv.render(*problem)
	}
	store, id, candidates := resolveAlertID(stores, named)
	switch {
	case len(candidates) > 1:
		argv := inv.publicArgv("alert", act, candidates[0])
		return inv.render(intentResult{Outcome: intentRefused, code: 2,
			Summary: fmt.Sprintf("%s names an alert in both %s, so nothing was done", named, strings.Join(candidates, " and ")),
			next:    argv, nextReason: "or the other one; the store is part of the id"})
	case store.root == "":
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: fmt.Sprintf("there is no alert %s, so nothing was done", named),
			next: inv.publicArgv("alert", "list", "--all"), nextReason: "names every alert"})
	}
	owners := inv.alerts()
	invoker, err := owners.invoker()
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error() + ", so nothing was done", retry: "the read may pass"})
	}
	qualified := store.name + "/" + id
	var episode steward.AlertEpisode
	changed := false
	if act == "ack" {
		var before steward.AlertEpisode
		if before, err = alertEpisode(store.root, id); err == nil {
			episode, err = steward.AcknowledgeAlert(store.root, id, invoker, owners.now())
			changed = !before.Acknowledged
		}
	} else {
		episode, changed, err = steward.ClearAlert(store.root, id, invoker, owners.now())
	}
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: fmt.Sprintf("alert %s could not be changed: %v", qualified, err),
			retry: "once its store is readable again"})
	}
	result := intentResult{Outcome: intentConfirmed, Data: map[string]any{"alert": listedAlert{ID: qualified, Store: store.name, Checkout: store.root, Episode: episode}}}
	switch {
	case act == "ack" && changed:
		result.Summary = "alert " + qualified + " is marked seen; it stays open"
		result.next, result.nextReason = inv.publicArgv("alert", "clear", qualified), "resolves it once it is handled"
	case act == "ack":
		result.Outcome, result.Summary = intentUnchanged, "alert "+qualified+" was already marked seen"
	case changed:
		result.Summary = "alert " + qualified + " is cleared"
		if steward.IsPatternOwner(episode.Owner) {
			result.Summary += "; the same work reports again only after it reads clear"
		}
	default:
		result.Outcome, result.Summary = intentUnchanged, "alert "+qualified+" was already cleared"
	}
	return inv.render(result)
}

func alertEpisode(root, id string) (steward.AlertEpisode, error) {
	episodes, err := steward.AlertEpisodes(root)
	if err != nil {
		return steward.AlertEpisode{}, err
	}
	for _, episode := range episodes {
		if episode.EpisodeID == id {
			return episode, nil
		}
	}
	return steward.AlertEpisode{}, fmt.Errorf("there is no alert %s", id)
}

// resolveAlertID finds a named alert: a qualified id in its store, or a bare
// id that exists in exactly one store. More than one candidate is returned,
// qualified, when a bare id is in both.
func resolveAlertID(stores []alertStore, named string) (alertStore, string, []string) {
	if prefix, id, qualified := strings.Cut(named, "/"); qualified {
		// On the lane checkout both names are one store.
		for _, store := range stores {
			if store.name == prefix || (prefix == "here" && len(stores) == 1) {
				if _, err := alertEpisode(store.root, id); err == nil {
					return store, id, nil
				}
			}
		}
		return alertStore{}, "", nil
	}
	var found []alertStore
	for _, store := range stores {
		if _, err := alertEpisode(store.root, named); err == nil {
			found = append(found, store)
		}
	}
	switch len(found) {
	case 0:
		return alertStore{}, "", nil
	case 1:
		return found[0], named, nil
	}
	var candidates []string
	for _, store := range found {
		candidates = append(candidates, store.name+"/"+named)
	}
	return alertStore{}, "", candidates
}

// openPatternAlerts counts the open behaviour-pattern alerts session status
// names on its first line: this installation's store and the host lane's,
// each once. A store that cannot be read counts nothing.
func openPatternAlerts(installation string, home func() (string, error)) int {
	roots := []string{installation}
	if host, err := home(); err == nil {
		if record, registered, err := lane.Read(host); err == nil && registered {
			if layout, err := stateroot.ResolveLayout(record.Root); err == nil {
				if root, err := stateroot.RootForInstallation(layout.InstallationRoot); err == nil &&
					realpath.Resolve(root.Path()) != realpath.Resolve(installation) {
					roots = append(roots, root.Path())
				}
			}
		}
	}
	count := 0
	for _, root := range roots {
		if open, err := steward.OpenPatternEpisodes(root); err == nil {
			count += open
		}
	}
	return count
}

// patternAlertAttention is session status's first line when a pattern alert
// is open.
func patternAlertAttention(open int) (textui.Attention, bool) {
	if open == 0 {
		return textui.Attention{}, false
	}
	text := "1 behaviour alert is open"
	if open > 1 {
		text = fmt.Sprintf("%d behaviour alerts are open", open)
	}
	return textui.Attention{State: textui.Alert, Text: text,
		Hint: textui.Hint{Argv: []string{"metasystem", "alert", "list"}, Reason: "shows what the steward saw"}}, true
}
