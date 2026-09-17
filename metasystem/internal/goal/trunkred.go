package goal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	TrunkRedBranchOpen   = "open"
	TrunkRedBranchMerged = "merged"
	trunkRedPath         = "plans/goals/trunk-red.json"
	trunkRedTimeLayout   = "2006-01-02T15:04:05Z"
)

type TrunkRedFailure struct {
	Report    string `json:"report"`
	Classname string `json:"classname"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
}

type TrunkRedSighting struct {
	Attempt    string `json:"attempt"`
	Batch      string `json:"batch"`
	BaseCommit string `json:"baseCommit"`
	BaseTree   string `json:"baseTree"`
	LogPath    string `json:"logPath"`
	LogDigest  string `json:"logDigest"`
	SeenAt     string `json:"seenAt"`
	Opid       string `json:"opid"`
}

type TrunkRedOwner struct {
	Machine string `json:"machine"`
	Since   string `json:"since"`
	How     string `json:"how"`
	By      string `json:"by"`
}

type TrunkRedBranch struct {
	Name   string `json:"name"`
	Commit string `json:"commit"`
	State  string `json:"state"`
}

func (branch TrunkRedBranch) Validate() error {
	if branch.Name == "" {
		if branch.Commit != "" || branch.State != "" {
			return fmt.Errorf("trunk-red branch without a name")
		}
		return nil
	}
	if branch.Commit == "" {
		return fmt.Errorf("trunk-red branch %q has no commit", branch.Name)
	}
	if branch.State != TrunkRedBranchOpen && branch.State != TrunkRedBranchMerged {
		return fmt.Errorf("trunk-red branch %q has unknown state %q", branch.Name, branch.State)
	}
	return nil
}

type TrunkRedClosure struct {
	At         string `json:"at"`
	Attempt    string `json:"attempt"`
	BaseCommit string `json:"baseCommit"`
	How        string `json:"how"`
	Opid       string `json:"opid"`
	By         string `json:"by"`
	Why        string `json:"why"`
}

type TrunkRedEntry struct {
	ID           string             `json:"id"`
	Identity     string             `json:"identity"`
	Group        string             `json:"group"`
	Status       string             `json:"status"`
	Failures     []TrunkRedFailure  `json:"failures"`
	NotRunReason string             `json:"notRunReason"`
	Sightings    []TrunkRedSighting `json:"sightings"`
	Owner        TrunkRedOwner      `json:"owner"`
	FixGoal      string             `json:"fixGoal"`
	FixBranch    TrunkRedBranch     `json:"fixBranch"`
	Holds        []string           `json:"holds"`
	Opened       string             `json:"opened"`
	Closed       *TrunkRedClosure   `json:"closed"`
}

type trunkRedFile struct {
	Schema       int             `json:"schema"`
	Entries      []TrunkRedEntry `json:"entries"`
	Cadence      *CadenceStatus  `json:"cadence,omitempty"`
	CadenceClaim *CadenceClaim   `json:"cadenceClaim,omitempty"`
}

// ParseTrunkRed validates the complete register before any reader can act on it.
func ParseTrunkRed(data []byte) ([]TrunkRedEntry, []Problem) {
	var file trunkRedFile
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, []Problem{trunkRedProblem("invalid JSON: %v", err)}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, []Problem{trunkRedProblem("trailing JSON data")}
	}
	var problems []Problem
	addf := func(format string, args ...any) { problems = append(problems, trunkRedProblem(format, args...)) }
	if file.Schema != 1 {
		addf("schema must be 1")
	}
	if file.Entries == nil {
		addf("entries must be an array")
	}
	if file.Cadence != nil {
		if err := validateCadenceStatus(file.Cadence); err != nil {
			addf("%v", err)
		}
	}
	if file.CadenceClaim != nil {
		if err := validateCadenceClaim(file.CadenceClaim); err != nil {
			addf("%v", err)
		}
	}
	if file.Cadence != nil && file.CadenceClaim != nil && file.Cadence.Key() == file.CadenceClaim.Key {
		addf("cadence status and claim have the same terminal key")
	}
	ids := map[string]bool{}
	openIdentities := map[string]bool{}
	for index := range file.Entries {
		entry := &file.Entries[index]
		label := fmt.Sprintf("entry %d", index+1)
		if entry.ID == "" || entry.Identity == "" || entry.Group == "" || entry.Status == "" {
			addf("%s needs non-empty id, identity, group, and status", label)
		}
		if !validTrunkRedID(entry.ID, entry.Identity) {
			addf("%s id %q does not derive from identity %q", label, entry.ID, entry.Identity)
		}
		if ids[entry.ID] {
			addf("entry id %q appears more than once", entry.ID)
		}
		ids[entry.ID] = true
		if entry.Failures == nil {
			addf("%s failures must be an array", label)
		}
		for failureIndex, failure := range entry.Failures {
			if failure.Report == "" || failure.Classname == "" || failure.Name == "" {
				addf("%s failure %d needs report, classname, and name", label, failureIndex+1)
			}
		}
		if len(entry.Sightings) == 0 {
			addf("%s needs at least one sighting", label)
		}
		for sightingIndex, sighting := range entry.Sightings {
			if sighting.Attempt == "" || sighting.BaseCommit == "" || sighting.SeenAt == "" || sighting.Opid == "" {
				addf("%s sighting %d needs attempt, base commit, seen at, and opid", label, sightingIndex+1)
			}
			if !validTrunkRedTime(sighting.SeenAt) {
				addf("%s sighting %d has invalid seenAt %q", label, sightingIndex+1, sighting.SeenAt)
			}
			if !validOpidShape(sighting.Opid) {
				addf("%s sighting %d has invalid opid %q", label, sightingIndex+1, sighting.Opid)
			}
		}
		validateTrunkRedOwner(label, entry.Owner, addf)
		if err := entry.FixBranch.Validate(); err != nil {
			addf("%s: %v", label, err)
		}
		seenHolds := map[string]bool{}
		if entry.Holds == nil {
			addf("%s holds must be an array", label)
		}
		for _, hold := range entry.Holds {
			if seenHolds[hold] {
				addf("%s holds batch %q more than once", label, hold)
			}
			seenHolds[hold] = true
		}
		if !validTrunkRedTime(entry.Opened) {
			addf("%s has invalid opened time %q", label, entry.Opened)
		}
		if entry.Closed == nil {
			if openIdentities[entry.Identity] {
				addf("identity %q has more than one open entry", entry.Identity)
			}
			openIdentities[entry.Identity] = true
		} else {
			validateTrunkRedClosure(label, entry.Closed, addf)
		}
		if stringContainsLineBreak(entry) {
			addf("%s contains a carriage return or line feed", label)
		}
		if index > 0 {
			prior := file.Entries[index-1]
			if prior.Opened > entry.Opened || prior.Opened == entry.Opened && prior.ID > entry.ID {
				addf("entries are not ordered by opened time and id")
			}
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return file.Entries, nil
}

func trunkRedProblem(format string, args ...any) Problem {
	return Problem(trunkRedPath + ": " + fmt.Sprintf(format, args...))
}

func validTrunkRedID(id, identity string) bool {
	if id == identity {
		return identity != ""
	}
	prefix := identity + "-"
	if identity == "" || !strings.HasPrefix(id, prefix) {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, prefix))
	return err == nil && n >= 2 && id == prefix+strconv.Itoa(n)
}

func validTrunkRedTime(value string) bool {
	parsed, err := time.Parse(trunkRedTimeLayout, value)
	return err == nil && parsed.UTC().Format(trunkRedTimeLayout) == value
}

func validateTrunkRedOwner(label string, owner TrunkRedOwner, addf func(string, ...any)) {
	if owner == (TrunkRedOwner{}) {
		return
	}
	if owner.Machine == "" || !validTrunkRedTime(owner.Since) {
		addf("%s owner needs a machine and valid since time", label)
	}
	if owner.How != "joiner" && owner.How != "taken" && owner.How != "hand" {
		addf("%s owner has unknown how %q", label, owner.How)
	}
	if (owner.How == "hand") != (owner.By != "") {
		addf("%s owner by is present exactly when how is hand", label)
	}
}

func validateTrunkRedClosure(label string, closed *TrunkRedClosure, addf func(string, ...any)) {
	if !validTrunkRedTime(closed.At) {
		addf("%s closure has invalid at time %q", label, closed.At)
	}
	if !validOpidShape(closed.Opid) {
		addf("%s closure has invalid opid %q", label, closed.Opid)
	}
	switch closed.How {
	case "green":
		if closed.Attempt == "" || closed.BaseCommit == "" || closed.By != "" || closed.Why != "" {
			addf("%s green closure needs attempt and base commit, and no human fields", label)
		}
	case "hand":
		if closed.By == "" || closed.Why == "" || closed.Attempt != "" || closed.BaseCommit != "" {
			addf("%s hand closure needs by and why, and no proof fields", label)
		}
	default:
		addf("%s closure has unknown how %q", label, closed.How)
	}
}

func stringContainsLineBreak(entry *TrunkRedEntry) bool {
	stringsToCheck := []string{entry.ID, entry.Identity, entry.Group, entry.Status, entry.NotRunReason,
		entry.Owner.Machine, entry.Owner.Since, entry.Owner.How, entry.Owner.By, entry.FixGoal,
		entry.FixBranch.Name, entry.FixBranch.Commit, entry.FixBranch.State, entry.Opened}
	for _, failure := range entry.Failures {
		stringsToCheck = append(stringsToCheck, failure.Report, failure.Classname, failure.Name, failure.Status, failure.Reason)
	}
	for _, sighting := range entry.Sightings {
		stringsToCheck = append(stringsToCheck, sighting.Attempt, sighting.Batch, sighting.BaseCommit, sighting.BaseTree,
			sighting.LogPath, sighting.LogDigest, sighting.SeenAt, sighting.Opid)
	}
	stringsToCheck = append(stringsToCheck, entry.Holds...)
	if entry.Closed != nil {
		stringsToCheck = append(stringsToCheck, entry.Closed.At, entry.Closed.Attempt, entry.Closed.BaseCommit,
			entry.Closed.How, entry.Closed.Opid, entry.Closed.By, entry.Closed.Why)
	}
	for _, value := range stringsToCheck {
		if strings.ContainsAny(value, "\r\n") {
			return true
		}
	}
	return false
}

// RenderTrunkRed is the register's only serializer.
func RenderTrunkRed(entries []TrunkRedEntry) []byte {
	copyEntries := make([]TrunkRedEntry, len(entries))
	for index := range entries {
		copyEntries[index] = cleanTrunkRedEntry(entries[index])
	}
	sort.Slice(copyEntries, func(i, j int) bool {
		if copyEntries[i].Opened == copyEntries[j].Opened {
			return copyEntries[i].ID < copyEntries[j].ID
		}
		return copyEntries[i].Opened < copyEntries[j].Opened
	})
	data, _ := json.MarshalIndent(trunkRedFile{Schema: 1, Entries: copyEntries}, "", "  ")
	return append(data, '\n')
}

func cleanTrunkRedEntry(entry TrunkRedEntry) TrunkRedEntry {
	clean := func(value string) string { return strings.NewReplacer("\r", " ", "\n", " ").Replace(value) }
	entry.ID, entry.Identity, entry.Group, entry.Status = clean(entry.ID), clean(entry.Identity), clean(entry.Group), clean(entry.Status)
	entry.NotRunReason, entry.FixGoal, entry.Opened = clean(entry.NotRunReason), clean(entry.FixGoal), clean(entry.Opened)
	entry.Owner = TrunkRedOwner{Machine: clean(entry.Owner.Machine), Since: clean(entry.Owner.Since), How: clean(entry.Owner.How), By: clean(entry.Owner.By)}
	entry.FixBranch = TrunkRedBranch{Name: clean(entry.FixBranch.Name), Commit: clean(entry.FixBranch.Commit), State: clean(entry.FixBranch.State)}
	entry.Failures = append([]TrunkRedFailure(nil), entry.Failures...)
	if entry.Failures == nil {
		entry.Failures = []TrunkRedFailure{}
	}
	for index := range entry.Failures {
		failure := &entry.Failures[index]
		failure.Report, failure.Classname, failure.Name = clean(failure.Report), clean(failure.Classname), clean(failure.Name)
		failure.Status, failure.Reason = clean(failure.Status), clean(failure.Reason)
	}
	entry.Sightings = append([]TrunkRedSighting(nil), entry.Sightings...)
	for index := range entry.Sightings {
		sighting := &entry.Sightings[index]
		sighting.Attempt, sighting.Batch = clean(sighting.Attempt), clean(sighting.Batch)
		sighting.BaseCommit, sighting.BaseTree = clean(sighting.BaseCommit), clean(sighting.BaseTree)
		sighting.LogPath, sighting.LogDigest = clean(sighting.LogPath), clean(sighting.LogDigest)
		sighting.SeenAt, sighting.Opid = clean(sighting.SeenAt), clean(sighting.Opid)
	}
	entry.Holds = append([]string(nil), entry.Holds...)
	if entry.Holds == nil {
		entry.Holds = []string{}
	}
	for index := range entry.Holds {
		entry.Holds[index] = clean(entry.Holds[index])
	}
	if entry.Closed != nil {
		closed := *entry.Closed
		closed.At, closed.Attempt, closed.BaseCommit = clean(closed.At), clean(closed.Attempt), clean(closed.BaseCommit)
		closed.How, closed.Opid, closed.By, closed.Why = clean(closed.How), clean(closed.Opid), clean(closed.By), clean(closed.Why)
		entry.Closed = &closed
	}
	return entry
}

type TrunkRedRecordArgs struct {
	Batch            string                `json:"batch"`
	Attempt          string                `json:"attempt"`
	BaseCommit       string                `json:"baseCommit"`
	BaseTree         string                `json:"baseTree"`
	SeenAt           string                `json:"seenAt"`
	OwnerMachine     string                `json:"ownerMachine"`
	Groups           []TrunkRedRecordGroup `json:"groups"`
	Cadence          *CadenceStatus        `json:"cadence,omitempty"`
	CadenceClaim     *CadenceClaim         `json:"cadenceClaim,omitempty"`
	CadenceClaimOpid string                `json:"cadenceClaimOpid,omitempty"`
}

type TrunkRedRecordGroup struct {
	Identity     string            `json:"identity"`
	Group        string            `json:"group"`
	Status       string            `json:"status"`
	NotRunReason string            `json:"notRunReason"`
	LogPath      string            `json:"logPath"`
	LogDigest    string            `json:"logDigest"`
	Failures     []TrunkRedFailure `json:"failures"`
}

// EntryRef identifies one register entry and the proof group it represents.
type EntryRef struct {
	ID    string
	Group string
}

// RecordTrunkRed publishes one batch observation through the goal transaction journal.
func RecordTrunkRed(r VerbRequest, args TrunkRedRecordArgs) (PublishResult, error) {
	return Publish(r.Endpoint, trunkRedRecordRequest(r, args))
}

func trunkRedRecordRequest(r VerbRequest, args TrunkRedRecordArgs) PublishRequest {
	encoded, _ := json.Marshal(args)
	groupNames := make([]string, len(args.Groups))
	for index, group := range args.Groups {
		groupNames[index] = group.Group
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "trunk-red-record", Args: map[string]string{"red": string(encoded)}},
		Message: "trunk-red record " + args.Batch + " " + strings.Join(groupNames, ","),
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			handled, cadenceAlready, err := applyCadenceRecord(tree, r, args)
			if err != nil {
				return nil, err
			}
			if handled {
				if cadenceAlready {
					return nil, AlreadyApplied{}
				}
				return []Change{{Path: trunkRedPath, Content: renderTrunkRedState(tree.TrunkRed, tree.Cadence, tree.CadenceClaim)}}, nil
			}
			already := 0
			for _, group := range args.Groups {
				if trunkRedSightingExists(tree.TrunkRed, group.Identity, r.opid()) {
					already++
				}
			}
			if already == len(args.Groups) && (len(args.Groups) > 0 || cadenceAlready) {
				return nil, AlreadyApplied{}
			}
			for _, group := range args.Groups {
				if trunkRedSightingExists(tree.TrunkRed, group.Identity, r.opid()) {
					continue
				}
				sighting := TrunkRedSighting{Attempt: args.Attempt, Batch: args.Batch, BaseCommit: args.BaseCommit,
					BaseTree: args.BaseTree, LogPath: group.LogPath, LogDigest: group.LogDigest, SeenAt: args.SeenAt, Opid: r.opid()}
				entry := openTrunkRedByIdentity(tree.TrunkRed, group.Identity)
				if entry == nil {
					id := group.Identity
					count := 0
					for _, prior := range tree.TrunkRed {
						if prior.Identity == group.Identity {
							count++
						}
					}
					if count > 0 {
						id = fmt.Sprintf("%s-%d", group.Identity, count+1)
					}
					owner := TrunkRedOwner{}
					if args.OwnerMachine != "" {
						owner = TrunkRedOwner{Machine: args.OwnerMachine, Since: args.SeenAt, How: "joiner"}
					}
					holds := []string{}
					if args.Batch != "" {
						holds = append(holds, args.Batch)
					}
					tree.TrunkRed = append(tree.TrunkRed, TrunkRedEntry{ID: id, Identity: group.Identity, Group: group.Group,
						Status: group.Status, Failures: append([]TrunkRedFailure(nil), group.Failures...), NotRunReason: group.NotRunReason,
						Sightings: []TrunkRedSighting{sighting}, Owner: owner, Holds: holds, Opened: args.SeenAt})
					continue
				}
				entry.Group, entry.Status, entry.NotRunReason = group.Group, group.Status, group.NotRunReason
				entry.Failures = append([]TrunkRedFailure(nil), group.Failures...)
				entry.Sightings = append(entry.Sightings, sighting)
				if args.Batch != "" && !contains(entry.Holds, args.Batch) {
					entry.Holds = append(entry.Holds, args.Batch)
				}
				if entry.Owner == (TrunkRedOwner{}) && args.OwnerMachine != "" {
					entry.Owner = TrunkRedOwner{Machine: args.OwnerMachine, Since: args.SeenAt, How: "joiner"}
				}
			}
			return []Change{{Path: trunkRedPath, Content: renderTrunkRedState(tree.TrunkRed, tree.Cadence, tree.CadenceClaim)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

func trunkRedSightingExists(entries []TrunkRedEntry, identity, opid string) bool {
	for _, entry := range entries {
		if entry.Identity != identity {
			continue
		}
		for _, sighting := range entry.Sightings {
			if sighting.Opid == opid {
				return true
			}
		}
	}
	return false
}

func openTrunkRedByIdentity(entries []TrunkRedEntry, identity string) *TrunkRedEntry {
	for index := range entries {
		if entries[index].Identity == identity && entries[index].Closed == nil {
			return &entries[index]
		}
	}
	return nil
}

// TrunkRedRefs returns the entries created or updated by one recording operation.
func TrunkRedRefs(tree *TreeGoals, opid string) []EntryRef {
	if tree == nil {
		return nil
	}
	var refs []EntryRef
	for _, entry := range tree.TrunkRed {
		for _, sighting := range entry.Sightings {
			if sighting.Opid == opid {
				refs = append(refs, EntryRef{ID: entry.ID, Group: entry.Group})
				break
			}
		}
	}
	return refs
}

type TrunkRedClearArgs struct {
	Entry        string `json:"entry"`
	Attempt      string `json:"attempt"`
	BaseCommit   string `json:"baseCommit"`
	BaseTree     string `json:"baseTree"`
	Group        string `json:"group"`
	BranchMerged bool   `json:"branchMerged"`
}

// ClearTrunkRed closes an entry after a green proof.
func ClearTrunkRed(r VerbRequest, args TrunkRedClearArgs) (PublishResult, error) {
	return Publish(r.Endpoint, trunkRedClearRequest(r, args))
}

func trunkRedClearRequest(r VerbRequest, args TrunkRedClearArgs) PublishRequest {
	intentArgs := map[string]string{"entry": args.Entry, "attempt": args.Attempt, "baseCommit": args.BaseCommit,
		"baseTree": args.BaseTree, "group": args.Group, "branchMerged": strconv.FormatBool(args.BranchMerged)}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "trunk-red-clear", Targets: []string{args.Entry}, Args: intentArgs},
		Message: "trunk-red clear " + args.Entry,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			entry := trunkRedByID(tree.TrunkRed, args.Entry)
			if entry == nil {
				return nil, fmt.Errorf("TRUNK_RED_UNKNOWN: entry %s is not in the register", args.Entry)
			}
			if entry.Closed != nil {
				if entry.Closed.Opid == r.opid() {
					return nil, AlreadyApplied{}
				}
				return nil, fmt.Errorf("TRUNK_RED_CLOSED: entry %s is closed", args.Entry)
			}
			entry.Closed = &TrunkRedClosure{At: r.stamp(), Attempt: args.Attempt, BaseCommit: args.BaseCommit, How: "green", Opid: r.opid()}
			entry.Holds = []string{}
			if args.BranchMerged && entry.FixBranch.Name != "" {
				entry.FixBranch.State = TrunkRedBranchMerged
			}
			return []Change{{Path: trunkRedPath, Content: renderTrunkRedState(tree.TrunkRed, tree.Cadence, tree.CadenceClaim)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

type TrunkRedOwnArgs struct {
	Entry        string `json:"entry"`
	Goal         string `json:"goal"`
	Branch       string `json:"branch"`
	BranchCommit string `json:"branchCommit"`
	To           string `json:"to"`
	By           string `json:"by"`
}

// OwnTrunkRed assigns the machine responsible for an open entry and its fix goal.
func OwnTrunkRed(r VerbRequest, args TrunkRedOwnArgs) (PublishResult, error) {
	if (args.By == "") != (r.Actor.Human == "") || args.By != "" && args.By != r.Actor.Human {
		return PublishResult{}, fmt.Errorf("trunk-red own human form must carry the same named human in the actor and --by")
	}
	return Publish(r.Endpoint, trunkRedOwnRequest(r, args))
}

func trunkRedOwnRequest(r VerbRequest, args TrunkRedOwnArgs) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "trunk-red-own", Targets: []string{args.Entry}, Args: map[string]string{
			"goal": args.Goal, "branch": args.Branch, "branchCommit": args.BranchCommit, "to": args.To, "by": args.By,
		}},
		Message: "trunk-red own " + args.Entry,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			entry := trunkRedByID(tree.TrunkRed, args.Entry)
			if entry == nil {
				return nil, fmt.Errorf("TRUNK_RED_UNKNOWN: entry %s is not in the register", args.Entry)
			}
			if entry.Closed != nil {
				return nil, fmt.Errorf("TRUNK_RED_CLOSED: entry %s is closed", args.Entry)
			}
			if tree.Live[args.Goal] == nil {
				return nil, fmt.Errorf("TRUNK_RED_FIX_GOAL_UNKNOWN: goal %s is not live", args.Goal)
			}
			if args.By == "" {
				if entry.Owner.Machine != "" && entry.Owner.Machine != r.Actor.Machine {
					return nil, fmt.Errorf("TRUNK_RED_OWNED_ELSEWHERE: entry %s is owned by %s", args.Entry, entry.Owner.Machine)
				}
				since := entry.Owner.Since
				if since == "" {
					since = r.stamp()
				}
				entry.Owner = TrunkRedOwner{Machine: r.Actor.Machine, Since: since, How: "taken"}
			} else {
				machine := args.To
				if machine == "" {
					machine = r.Actor.Machine
				}
				entry.Owner = TrunkRedOwner{Machine: machine, Since: r.stamp(), How: "hand", By: args.By}
			}
			entry.FixGoal = args.Goal
			if args.Branch != "" {
				entry.FixBranch = TrunkRedBranch{Name: args.Branch, Commit: args.BranchCommit, State: TrunkRedBranchOpen}
			}
			return []Change{{Path: trunkRedPath, Content: renderTrunkRedState(tree.TrunkRed, tree.Cadence, tree.CadenceClaim)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

type TrunkRedCloseArgs struct {
	Entry string `json:"entry"`
	By    string `json:"by"`
	Why   string `json:"why"`
}

// CloseTrunkRed records a person's explicit resolution of an open entry.
func CloseTrunkRed(r VerbRequest, args TrunkRedCloseArgs) (PublishResult, error) {
	if args.By == "" || r.Actor.Human == "" || args.By != r.Actor.Human {
		return PublishResult{}, fmt.Errorf("TRUNK_RED_CLOSE_IS_HUMAN: close names its human with --by")
	}
	return Publish(r.Endpoint, trunkRedCloseRequest(r, args))
}

func trunkRedCloseRequest(r VerbRequest, args TrunkRedCloseArgs) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "trunk-red-close", Targets: []string{args.Entry}, Args: map[string]string{"by": args.By, "why": args.Why}},
		Message: "trunk-red close " + args.Entry,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			entry := trunkRedByID(tree.TrunkRed, args.Entry)
			if entry == nil {
				return nil, fmt.Errorf("TRUNK_RED_UNKNOWN: entry %s is not in the register", args.Entry)
			}
			if entry.Closed != nil {
				return nil, fmt.Errorf("TRUNK_RED_CLOSED: entry %s is closed", args.Entry)
			}
			entry.Closed = &TrunkRedClosure{At: r.stamp(), How: "hand", Opid: r.opid(), By: args.By, Why: args.Why}
			entry.Holds = []string{}
			return []Change{{Path: trunkRedPath, Content: renderTrunkRedState(tree.TrunkRed, tree.Cadence, tree.CadenceClaim)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

func trunkRedByID(entries []TrunkRedEntry, id string) *TrunkRedEntry {
	for index := range entries {
		if entries[index].ID == id {
			return &entries[index]
		}
	}
	return nil
}
