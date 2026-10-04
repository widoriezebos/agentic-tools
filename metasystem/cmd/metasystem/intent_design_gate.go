package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/narratordigest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
)

type designGateOwners struct {
	chains   func(root, goal, path string) ([]designgate.Chain, error)
	identity func(root string) (string, error)
	record   func(path, text, anchor string) (durable bool, err error)
	digest   func(root string, entry narratordigest.Entry, now time.Time) error
}

type designGateRecord struct {
	Schema         int                    `json:"schema"`
	LedgerIdentity string                 `json:"ledgerIdentity"`
	Goal           string                 `json:"goal"`
	Unit           string                 `json:"unit"`
	Worktree       string                 `json:"worktree"`
	Tier           uint8                  `json:"tier"`
	Mode           string                 `json:"mode"`
	Verdict        string                 `json:"verdict"`
	WouldRefuse    bool                   `json:"wouldRefuse"`
	Person         bool                   `json:"person"`
	GovernedBy     string                 `json:"governedBy"`
	Time           time.Time              `json:"time"`
	Designs        []landing.DesignRecord `json:"designs"`
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
	f.Allowed = file != nil && file.DesignGateOff
	mode, _, code, err := inv.work().config(config.DesignGateModeKey, intentConfPath(inv.layout))
	if err == nil && code == 0 {
		f.Mode, err = config.ParseDesignGateMode(mode)
	} else if err == nil {
		err = fmt.Errorf("%s could not be read", config.DesignGateModeKey)
	}
	if err != nil {
		f.Error = err
		return f
	}
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

func (inv *intentInvocation) recordDesignGate(store, worktree, unit string, f designgate.Facts, result designgate.Result, person bool) {
	o := inv.designGate()
	now := time.Now().UTC()
	r := designGateRecord{Schema: 1, Goal: f.Goal, Unit: unit, Worktree: worktree, Tier: f.Tier, Mode: result.Mode,
		Verdict: result.Verdict, WouldRefuse: result.WouldRefuse, Person: person, GovernedBy: refusal.GovernedBy["BUILD_DESIGN_NOT_ACCEPTED"], Time: now, Designs: []landing.DesignRecord{}}
	var bodyErr error
	for _, d := range f.Designs {
		if d.Status == "accepted" {
			body, err := inv.designBodyDigest(d)
			if err != nil {
				bodyErr = err
			}
			r.Designs = append(r.Designs, landing.DesignRecord{Design: d, BodySHA256: body})
		}
	}
	identity, err := o.identity(inv.stateRoot)
	if bodyErr != nil {
		err = bodyErr
	}
	if err == nil && !designGateIdentity.MatchString(identity) {
		err = fmt.Errorf("the goal ledger identity is not 26 Crockford base32 characters")
	}
	if err == nil {
		for _, part := range []struct{ name, value string }{
			{"goal ledger identity", identity}, {"goal id", f.Goal}, {"work name", unit},
		} {
			if part.value == "" || part.value == "." || part.value == ".." || strings.ContainsAny(part.value, `/\`) {
				err = fmt.Errorf("the %s is not one plain path segment", part.name)
				break
			}
		}
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
		root, err := inv.designGateDigestRoot(worktree)
		if err == nil && root == "" {
			fmt.Fprintln(inv.stderr, "warning: the design check's digest line was left out, because the only digest is in the worktree under build and would enter its change; the build goes on\nnothing to do: the dispatch record holds the verdict")
		} else if err == nil {
			err = o.digest(root, entry, now)
		}
		if err != nil {
			inv.designGateWriteWarning(err)
		}
	}
}

// designGateDigestRoot is where a build's design-check line is written: the
// invocation's state root, unless that lies inside the worktree under build,
// whose tracked digest would then enter the unit's change. Then the line goes
// to the primary checkout, the first tree git worktree list names; when that
// is the worktree under build too, the root is empty and no line is written.
func (inv *intentInvocation) designGateDigestRoot(worktree string) (string, error) {
	tree := realpath.Resolve(worktree)
	if !realpath.Within(realpath.Resolve(inv.stateRoot), tree) {
		return inv.stateRoot, nil
	}
	output, err := inv.work().git(inv.layout.GitRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return "", fmt.Errorf("the primary checkout cannot be found: %w", err)
	}
	primary, _, _ := strings.Cut(string(output), "\n")
	primary, found := strings.CutPrefix(primary, "worktree ")
	if !found || primary == "" {
		return "", fmt.Errorf("the primary checkout cannot be found: git worktree list names no tree")
	}
	if realpath.Within(realpath.Resolve(primary), tree) {
		return "", nil
	}
	return primary, nil
}

func (inv *intentInvocation) designGateWriteWarning(err error) string {
	line := fmt.Sprintf("warning: the design check's record could not be written (%s); the build goes on", strings.ReplaceAll(err.Error(), "\n", " "))
	fmt.Fprintln(inv.stderr, line+"\nnothing to do: the landing check runs without it")
	return line
}

func (inv *intentInvocation) designBodyDigest(d designgate.Design) (string, error) {
	path := filepath.FromSlash(d.Path)
	if !filepath.IsAbs(path) {
		path = filepath.Join(inv.layout.GitRoot, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != d.SHA256 {
		return "", fmt.Errorf("the design %s changed while its facts were read", d.Path)
	}
	record, _, declared := project.ParseRecord(d.Path, string(data))
	if !declared || len(record.Head) == 0 {
		return "", fmt.Errorf("the design %s cannot be read", d.Path)
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	body := lines[record.Head[len(record.Head)-1].Line:]
	if len(body) > 0 && strings.TrimSpace(body[0]) == "" {
		body = body[1:]
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(body, "\n")))), nil
}

func (inv *intentInvocation) landingDesignFacts(root, id string) landing.DesignFacts {
	if id == "" {
		return landing.DesignFacts{Facts: designgate.Facts{Tier: 1}}
	}
	f := landing.DesignFacts{Facts: inv.designGateFacts(root, id), Digests: map[string]string{}}
	if designgate.Check(f.Facts).Verdict != "ok" {
		return f
	}
	for _, d := range f.Designs {
		if d.Status == "accepted" {
			digest, err := inv.designBodyDigest(d)
			if err != nil {
				f.Error = err
				return f
			}
			f.Digests[d.ID] = digest
		}
	}
	f.Recorded, f.RecordError = inv.readDesignGateRecords(id)
	return f
}

func (inv *intentInvocation) readDesignGateRecords(id string) ([]landing.DesignRecord, error) {
	identity, err := inv.designGate().identity(inv.stateRoot)
	if err != nil || !designGateIdentity.MatchString(identity) {
		return nil, nil
	}
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
		return nil, fmt.Errorf("the goal id is not one plain path segment")
	}
	runner := inv.work().units(inv.layout)
	store := runner.Root
	if store == "" {
		launchRoot := runner.Manager.Store.Root
		if launchRoot == "" {
			launchRoot, err = launch.DefaultRoot()
			if err != nil {
				return nil, err
			}
		}
		store = filepath.Join(filepath.Dir(launchRoot), "unit")
	}
	directory := filepath.Join(store, ".design-gate", identity, id)
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var designs []landing.DesignRecord
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		var record designGateRecord
		if err == nil {
			err = json.Unmarshal(data, &record)
		}
		if err != nil {
			return nil, err
		}
		if record.Schema != 1 || record.LedgerIdentity != identity || record.Goal != id || record.Unit+".json" != entry.Name() {
			return nil, fmt.Errorf("the design record %s does not belong to this goal and unit", entry.Name())
		}
		work, err := runner.NamedWork(record.Worktree, id)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, unit := range work {
			if unit.Unit == record.Unit && unit.Run != "" && unit.Record != nil {
				for _, d := range record.Designs {
					if d.BodySHA256 != "" {
						designs = append(designs, d)
					}
				}
				break
			}
		}
	}
	return designs, nil
}

func landingDesignInvocation(root string, stderr io.Writer) (*intentInvocation, error) {
	inv := &intentInvocation{owners: defaultIntentOwners(), cwd: root, stderr: stderr}
	layout, err := inv.owners.resolver.ResolveLayout(root)
	if err == nil {
		inv.layout = layout
		stateRoot, rootErr := inv.owners.resolver.RootForInstallation(layout.InstallationRoot)
		inv.stateRoot, err = string(stateRoot), rootErr
	}
	return inv, err
}

func (inv *intentInvocation) recordLandingDesign(id string, design *landing.DesignObservation, now time.Time) {
	if id == "" || design == nil || design.Pair[0] == "" {
		return
	}
	tip, err := inv.work().git(string(inv.layout.InstallationRoot), "rev-parse", "HEAD")
	if err == nil {
		err = inv.designGate().digest(inv.stateRoot, narratordigest.Entry{Kind: "lowlight", Text: design.Pair[0],
			SourceType: "design-gate-landing", SourceID: id + "@" + strings.TrimSpace(string(tip))}, now)
	}
	if err != nil {
		landingDesignDigestWarning(inv.stderr, err)
	}
}

func landingDesignDigestWarning(stderr io.Writer, err error) {
	fmt.Fprintf(stderr, "warning: the design check's digest could not be written (%s); the landing goes on\nnothing to do: the landing check's verdict still stands\n", strings.ReplaceAll(err.Error(), "\n", " "))
}
