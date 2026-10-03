package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
)

type designGateOwners struct {
	chains   func(root, goal, path string) ([]designgate.Chain, error)
	identity func(root string) (string, error)
	record   func(path, text, anchor string) (durable bool, err error)
	digest   func(root string, entry narratordigest.Entry, now time.Time) error
}

type designGateRecord struct {
	Schema         int                 `json:"schema"`
	LedgerIdentity string              `json:"ledgerIdentity"`
	Goal           string              `json:"goal"`
	Unit           string              `json:"unit"`
	Worktree       string              `json:"worktree"`
	Tier           uint8               `json:"tier"`
	Mode           string              `json:"mode"`
	Verdict        string              `json:"verdict"`
	WouldRefuse    bool                `json:"wouldRefuse"`
	GovernedBy     string              `json:"governedBy"`
	Time           time.Time           `json:"time"`
	Designs        []designgate.Design `json:"designs"`
}

func (inv *intentInvocation) designGate() designGateOwners {
	o := inv.work().designGate
	if o.chains == nil {
		o.chains = func(root, id, path string) ([]designgate.Chain, error) {
			read, err := dispatchcore.ReadDesignCritiqueChains(root, id, path)
			var chains []designgate.Chain
			for _, c := range read {
				chains = append(chains, designgate.Chain{Closed: c.Closed, Round: c.NewestRound})
			}
			return chains, err
		}
	}
	if o.identity == nil {
		o.identity = func(root string) (string, error) {
			endpoint, err := inv.owners.dependencies.endpoint(root)
			if err != nil {
				return "", err
			}
			if identity := goal.ExistingLedgerIdentityAtEndpoint(endpoint); identity != "" {
				return identity, nil
			}
			data, err := os.ReadFile(filepath.Join(root, "plans", "goals", "backlog.md"))
			if err != nil {
				return "", err
			}
			record, problems := goal.ParseRoot(data)
			if len(problems) > 0 {
				return "", fmt.Errorf("the goal ledger root cannot be read: %v", problems)
			}
			return record.Identity, nil
		}
	}
	if o.record == nil {
		o.record = atomicfile.WriteText
	}
	if o.digest == nil {
		o.digest = func(root string, entry narratordigest.Entry, now time.Time) error {
			return narratordigest.AppendWithLayoutReader(root, []narratordigest.Entry{entry}, now, inv.owners.resolver.ResolveLayout)
		}
	}
	return o
}

func (inv *intentInvocation) designGateFacts(root, id string) designgate.Facts {
	f := designgate.Facts{Goal: id}
	projection, _, problem := inv.projection()
	if problem != nil {
		f.Error = fmt.Errorf("%s", problem.Summary)
		return f
	}
	file, _ := goalRecord(projection, id)
	f.Tier = goal.GateTier(file)
	designs, problemText := inv.linkedDesigns(id)
	if problemText != "" {
		f.Error = fmt.Errorf("%s", problemText)
		return f
	}
	for _, ref := range designs {
		d := designgate.Design{ID: ref.ID, Path: ref.Path, Name: ref.Title, Status: ref.Status, Critique: ref.Critique}
		if ref.Status == "accepted" {
			path := filepath.FromSlash(ref.Path)
			if !filepath.IsAbs(path) {
				path = filepath.Join(inv.layout.GitRoot, path)
			}
			data, err := os.ReadFile(path)
			if err == nil {
				d.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
				d.Chains, err = inv.designGate().chains(root, id, path)
			}
			if err != nil {
				f.Error = err
			}
		}
		f.Designs = append(f.Designs, d)
	}
	return f
}

var designGateIdentity = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func (inv *intentInvocation) recordDesignGate(store, worktree, unit string, f designgate.Facts, result designgate.Result) {
	o := inv.designGate()
	now := time.Now().UTC()
	r := designGateRecord{Schema: 1, Goal: f.Goal, Unit: unit, Worktree: worktree, Tier: f.Tier, Mode: result.Mode,
		Verdict: result.Verdict, WouldRefuse: result.WouldRefuse, Time: now, Designs: []designgate.Design{}}
	for _, d := range f.Designs {
		if d.Status == "accepted" {
			r.Designs = append(r.Designs, d)
		}
	}
	identity, err := o.identity(inv.stateRoot)
	if err == nil && !designGateIdentity.MatchString(identity) {
		err = fmt.Errorf("the goal ledger identity is not 26 Crockford base32 characters")
	}
	if err == nil {
		r.LedgerIdentity = identity
		var data []byte
		data, err = json.MarshalIndent(r, "", "  ")
		if err == nil {
			path := filepath.Join(store, ".design-gate", identity, f.Goal, unit+".json")
			var durable bool
			durable, err = o.record(path, string(data)+"\n", store)
			if err == nil && !durable {
				err = fmt.Errorf("the design record's durability is unknown")
			}
		}
	}
	line := result.Warning[0]
	if err != nil {
		line = inv.designGateWriteWarning(err)
	}
	if line != "" {
		entry := narratordigest.Entry{Kind: "lowlight", Text: line, SourceType: "design-gate", SourceID: f.Goal + "/" + unit}
		if err := o.digest(inv.stateRoot, entry, now); err != nil {
			inv.designGateWriteWarning(err)
		}
	}
}

func (inv *intentInvocation) designGateWriteWarning(err error) string {
	line := fmt.Sprintf("warning: the design check's record could not be written (%s); the build goes on", strings.ReplaceAll(err.Error(), "\n", " "))
	fmt.Fprintln(inv.stderr, line+"\nnothing to do: the landing check runs without it")
	return line
}
