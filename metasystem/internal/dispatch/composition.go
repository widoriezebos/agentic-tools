package dispatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

const rolePacketTablePath = "scripts/agents/role-packets.json"

// CompositionRefusal is a typed brief-assembly refusal. The source is kept
// separate from the explanation so callers never decide from prose.
type CompositionRefusal struct {
	Code   string `json:"reasonCode"`
	Source string `json:"source,omitempty"`
	Detail string `json:"detail"`
}

func (e *CompositionRefusal) Error() string { return e.Detail }

type rolePacketTable struct {
	SchemaVersion    int                                      `json:"schemaVersion"`
	DestructiveReach map[HazardClass]ConfigurationObligations `json:"destructiveReach"`
	Roles            map[string]rolePacketRecipe              `json:"roles"`
}

type rolePacketRecipe struct {
	Sources []rolePacketSource `json:"sources"`
}

type rolePacketSource struct {
	Slot string `json:"slot"`
	Path string `json:"path"`
}

// CompositionSource binds one delivered packet range to the bytes selected
// from its declared source.
type CompositionSource struct {
	Slot            string `json:"slot"`
	Source          string `json:"source"`
	SourceDigest    string `json:"sourceDigest"`
	DeliveredDigest string `json:"deliveredDigest"`
	SourceBytes     int    `json:"sourceBytes"`
	StartByte       int    `json:"startByte"`
	EndByte         int    `json:"endByte"`
}

// CompositionRecord is the packet provenance stored inside the job record
// and beside the delivered prompt.
type CompositionRecord struct {
	SchemaVersion            int                      `json:"schemaVersion"`
	JobID                    string                   `json:"jobId"`
	Role                     string                   `json:"role"`
	Runtime                  string                   `json:"runtime"`
	Model                    string                   `json:"model"`
	Round                    int64                    `json:"round"`
	Mission                  string                   `json:"mission"`
	DestructiveReach         HazardClass              `json:"destructiveReach"`
	ConfigurationObligations ConfigurationObligations `json:"configurationObligations"`
	Recipe                   string                   `json:"recipe"`
	RecipeDigest             string                   `json:"recipeDigest"`
	PacketDigest             string                   `json:"packetDigest"`
	ContextProof             ContextProof             `json:"contextProof"`
	ToolSurface              ToolSurfaceProof         `json:"toolSurface"`
	MachineSlot              MachineSlotAdmission     `json:"machineSlotAdmission"`
	Sources                  []CompositionSource      `json:"sources"`
}

// ContextProof states the current broad-read limitation without presenting a
// narrow prompt as proof of isolation.
type ContextProof struct {
	Classification string `json:"classification"`
	ProofState     string `json:"proofState"`
	ReasonCode     string `json:"reasonCode"`
}

// ToolSurfaceProof distinguishes an exact empty tool surface from a runtime
// whose launcher cannot observe the provider's tool-name catalog.
type ToolSurfaceProof struct {
	Policy    string   `json:"policy"`
	NameState string   `json:"nameState"`
	Names     []string `json:"names"`
}

// MachineSlotAdmission preserves the admission point owned by the future
// machine-concurrency governor without pretending that it is enforced here.
type MachineSlotAdmission struct {
	Outcome   string `json:"outcome"`
	OwnerGoal string `json:"ownerGoal"`
}

// ComposeRolePacketParams names every value that may enter the closed packet.
// ExtraSources are assertions from an outer caller; they never add bytes.
type ComposeRolePacketParams struct {
	Root              string
	Role              string
	Brief             string
	JobID             string
	Runtime           string
	Model             string
	Round             int64
	Mission           string
	DestructiveReach  HazardClass
	Output            string
	CompositionOutput string
	ToolPolicy        string
	ExtraSources      []string
	Continuations     []CompositionContinuation
}

// CompositionContinuation is engine-owned same-lineage context. Its slots
// are fixed; callers cannot use it as a free-form source grant.
type CompositionContinuation struct {
	Slot string
	Path string
}

// ComposeRolePacket builds the packet only from the fixed role recipe and the
// generated values named in the request. Any caller-selected source outside
// the recipe refuses before job publication.
func ComposeRolePacket(p ComposeRolePacketParams) (CompositionRecord, error) {
	if p.Root == "" || p.Role == "" || p.Brief == "" || p.JobID == "" || p.Runtime == "" || p.Model == "" || p.ToolPolicy == "" || p.Round < 1 || p.Output == "" || p.CompositionOutput == "" {
		return CompositionRecord{}, fmt.Errorf("role-packet composition requires root, role, brief, job, runtime, model, tool policy, round, output, and composition output")
	}
	tableBytes, recipe, err := ValidateRolePacketSources(p.Root, p.Role, p.ExtraSources)
	if err != nil {
		return CompositionRecord{}, err
	}
	configuration, err := ResolveHazardConfiguration(p.Root, p.DestructiveReach)
	if err != nil {
		return CompositionRecord{}, &CompositionRefusal{Code: "REFUSED-HAZARD-CONFIGURATION", Source: string(p.DestructiveReach), Detail: err.Error()}
	}
	if err := ValidateRuntimeHazardConfiguration(p.Root, p.Runtime, p.Model, p.DestructiveReach); err != nil {
		return CompositionRecord{}, &CompositionRefusal{Code: "REFUSED-HAZARD-CONFIGURATION", Source: p.Runtime, Detail: err.Error()}
	}

	briefBytes, err := os.ReadFile(p.Brief)
	if err != nil {
		return CompositionRecord{}, fmt.Errorf("read task direction: %w", err)
	}
	if !utf8.Valid(briefBytes) {
		return CompositionRecord{}, &CompositionRefusal{Code: "REFUSED-TASK-DIRECTION", Source: p.Brief, Detail: "task direction is not valid UTF-8"}
	}

	record := CompositionRecord{
		SchemaVersion: 1, JobID: p.JobID, Role: p.Role, Runtime: p.Runtime,
		Model: p.Model, Round: p.Round, Mission: emptyAsNone(p.Mission),
		DestructiveReach: p.DestructiveReach, ConfigurationObligations: configuration,
		Recipe: rolePacketTablePath + "#" + p.Role, RecipeDigest: digestBytes(tableBytes),
		ContextProof: ContextProof{
			Classification: "advisory", ProofState: "no-leak-not-proven", ReasonCode: "BROAD-READ-RUNTIME",
		},
		MachineSlot: MachineSlotAdmission{Outcome: "DEFERRED", OwnerGoal: "machine-concurrency-governor"},
	}

	var packet bytes.Buffer
	appendSource := func(slot, source string, raw []byte) {
		heading := "# " + packetHeading(slot) + "\n\n"
		delivered := append([]byte(heading), raw...)
		if len(delivered) == 0 || delivered[len(delivered)-1] != '\n' {
			delivered = append(delivered, '\n')
		}
		delivered = append(delivered, '\n')
		start := packet.Len()
		packet.Write(delivered)
		record.Sources = append(record.Sources, CompositionSource{
			Slot: slot, Source: source, SourceDigest: digestBytes(raw), DeliveredDigest: digestBytes(delivered),
			SourceBytes: len(raw), StartByte: start, EndByte: packet.Len(),
		})
	}

	appendSource("task-direction", "caller:brief", briefBytes)
	for _, source := range recipe.Sources {
		content, readErr := readRecipeSource(p.Root, source)
		if readErr != nil {
			return CompositionRecord{}, readErr
		}
		appendSource(source.Slot, source.Path, content)
	}
	toolNotice := fmt.Sprintf("Permission tool policy: %s\n", p.ToolPolicy)
	if p.Runtime == "fake" {
		record.ToolSurface = ToolSurfaceProof{Policy: p.ToolPolicy, NameState: "exact", Names: []string{}}
		toolNotice += "Tool-name observation: exact\nTool names: (none; the fake runtime opens no model tool channel)\n"
	} else {
		record.ToolSurface = ToolSurfaceProof{Policy: p.ToolPolicy, NameState: "unobserved", Names: []string{}}
		toolNotice += "Tool-name observation: unobserved\nTool names: UNOBSERVED; this broad-read launcher cannot prove the provider tool catalog, so this job is advisory.\n"
	}
	appendSource("tool-names", "generated:tool-names", []byte(toolNotice))
	identity := fmt.Sprintf("Job-Id: %s\nRole: %s\nRuntime: %s\nModel: %s\nRound: %d\nMission: %s\nDestructive reach class: %s\nBuilder effort tier: %s\nBuilder reasoning effort: %s\nIndependent critique required: %t\nIndependent critique effort tier: %s\nIndependent critique reasoning effort: %s\nLive proof required: %t\n",
		p.JobID, p.Role, p.Runtime, p.Model, p.Round, emptyAsNone(p.Mission), p.DestructiveReach,
		configuration.BuilderEffortTier, configuration.BuilderReasoningEffort,
		configuration.IndependentCritiqueRequired, configuration.IndependentCritiqueEffortTier,
		configuration.IndependentCritiqueReasoningEffort, configuration.LiveProofRequired)
	appendSource("generated-runtime-notice", "generated:runtime-notice", []byte(identity+"Context classification: advisory. This broad-read runtime does not prove context isolation or independent examination.\n"))
	seenContinuations := map[string]bool{}
	for _, continuation := range p.Continuations {
		if continuation.Slot != "prior-brief" && continuation.Slot != "prior-return" && continuation.Slot != "critique-register" {
			return CompositionRecord{}, &CompositionRefusal{Code: "REFUSED-CONTEXT-SOURCE", Source: continuation.Slot, Detail: fmt.Sprintf("role packet refused undeclared continuation slot %q", continuation.Slot)}
		}
		if continuation.Path == "" || seenContinuations[continuation.Slot] {
			return CompositionRecord{}, &CompositionRefusal{Code: "REFUSED-CONTEXT-SOURCE", Source: continuation.Path, Detail: fmt.Sprintf("role packet continuation %q is empty or duplicated", continuation.Slot)}
		}
		seenContinuations[continuation.Slot] = true
		content, readErr := os.ReadFile(continuation.Path)
		if readErr != nil {
			return CompositionRecord{}, fmt.Errorf("read %s continuation: %w", continuation.Slot, readErr)
		}
		if !utf8.Valid(content) {
			return CompositionRecord{}, &CompositionRefusal{Code: "REFUSED-CONTEXT-SOURCE", Source: continuation.Path, Detail: fmt.Sprintf("role packet continuation %q is not valid UTF-8", continuation.Slot)}
		}
		appendSource(continuation.Slot, "engine:"+continuation.Slot, content)
	}
	record.PacketDigest = digestBytes(packet.Bytes())

	recordBytes, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return CompositionRecord{}, err
	}
	recordBytes = append(recordBytes, '\n')
	if _, err := atomicfile.WriteText(p.Output, packet.String(), ""); err != nil {
		return CompositionRecord{}, fmt.Errorf("write composed role packet: %w", err)
	}
	if _, err := atomicfile.WriteText(p.CompositionOutput, string(recordBytes), ""); err != nil {
		return CompositionRecord{}, fmt.Errorf("write composition record: %w", err)
	}
	return record, nil
}

// ValidateRolePacketSources performs the complete caller-source admission
// check without reading a brief or writing packet state. Delegate runs this
// before a runtime probe or workspace mutation.
func ValidateRolePacketSources(root, role string, extraSources []string) ([]byte, rolePacketRecipe, error) {
	tableBytes, _, recipe, err := readRolePacketRecipe(root, role)
	if err != nil {
		return nil, rolePacketRecipe{}, err
	}
	allowed := make(map[string]bool, len(recipe.Sources))
	for _, source := range recipe.Sources {
		allowed[source.Path] = true
	}
	for _, source := range extraSources {
		clean := filepath.ToSlash(filepath.Clean(source))
		if filepath.IsAbs(source) || !allowed[clean] {
			return nil, rolePacketRecipe{}, &CompositionRefusal{
				Code: "REFUSED-CONTEXT-SOURCE", Source: source,
				Detail: fmt.Sprintf("role packet refused source %q because the %s recipe does not select it", source, role),
			}
		}
	}
	return tableBytes, recipe, nil
}

func readRolePacketRecipe(root, role string) ([]byte, rolePacketTable, rolePacketRecipe, error) {
	data, table, err := readRolePacketTable(root)
	if err != nil {
		return nil, rolePacketTable{}, rolePacketRecipe{}, err
	}
	recipe, ok := table.Roles[role]
	if !ok || len(recipe.Sources) == 0 {
		return nil, table, rolePacketRecipe{}, &CompositionRefusal{Code: "REFUSED-ROLE-RECIPE", Source: role, Detail: fmt.Sprintf("role packet table has no complete recipe for %q", role)}
	}
	seen := map[string]bool{}
	for _, source := range recipe.Sources {
		if source.Slot == "" || source.Path == "" || filepath.IsAbs(source.Path) || filepath.Clean(source.Path) != filepath.FromSlash(source.Path) || strings.HasPrefix(source.Path, ".."+string(filepath.Separator)) || seen[source.Path] {
			return nil, table, rolePacketRecipe{}, fmt.Errorf("role packet recipe %s has an invalid or duplicate source %q", role, source.Path)
		}
		seen[source.Path] = true
	}
	return data, table, recipe, nil
}

func readRolePacketTable(root string) ([]byte, rolePacketTable, error) {
	path := filepath.Join(root, filepath.FromSlash(rolePacketTablePath))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, rolePacketTable{}, fmt.Errorf("read role packet table: %w", err)
	}
	var table rolePacketTable
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, table, fmt.Errorf("decode role packet table: %w", err)
	}
	if table.SchemaVersion != 1 || len(table.Roles) == 0 || len(table.DestructiveReach) == 0 {
		return nil, table, fmt.Errorf("role packet table must be schema version 1 with destructiveReach classes and at least one role")
	}
	return data, table, nil
}

func readRecipeSource(root string, source rolePacketSource) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(source.Path))
	canonicalRoot := resolvePath(root)
	canonicalPath := resolvePath(path)
	if !pathWithin(canonicalPath, canonicalRoot) || canonicalPath == canonicalRoot {
		return nil, &CompositionRefusal{Code: "REFUSED-CONTEXT-SOURCE", Source: source.Path, Detail: fmt.Sprintf("role packet source %q escapes the metasystem root", source.Path)}
	}
	info, err := os.Stat(canonicalPath)
	if err != nil {
		return nil, fmt.Errorf("read role packet source %s: %w", source.Path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, &CompositionRefusal{Code: "REFUSED-CONTEXT-SOURCE", Source: source.Path, Detail: fmt.Sprintf("role packet source %q is not a regular file", source.Path)}
	}
	data, err := os.ReadFile(canonicalPath)
	if err != nil {
		return nil, fmt.Errorf("read role packet source %s: %w", source.Path, err)
	}
	if !utf8.Valid(data) {
		return nil, &CompositionRefusal{Code: "REFUSED-CONTEXT-SOURCE", Source: source.Path, Detail: fmt.Sprintf("role packet source %q is not valid UTF-8", source.Path)}
	}
	return data, nil
}

func packetHeading(slot string) string {
	words := strings.Split(strings.ReplaceAll(slot, "_", "-"), "-")
	for index := range words {
		if words[index] != "" {
			words[index] = strings.ToUpper(words[index][:1]) + words[index][1:]
		}
	}
	return strings.Join(words, " ")
}

func digestBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func emptyAsNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}
