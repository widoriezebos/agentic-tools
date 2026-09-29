package diskstore

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// A disposal of evidence (3.12 "Inventory before the first unlink", "The
// tombstone and the pointer", "Commit point and recovery", "The receipt";
// R20): every disposal, compaction or removal, by machinery or a person,
// first walks the item and writes every file's path, size and sha256 into a
// tombstone that is durable (file and directory synced) before anything
// else; a removal then renames the item aside; the receipt line appended
// and synced to evidence.root/disposals/<segment>.jsonl is the commit
// point; only then are members unlinked, and the tombstone is marked done.
// A restart before the receipt re-judges and continues or rolls back; a
// restart after it finishes with exactly that one receipt. The caller holds
// the bound lock and, for a chain, its job lifecycle locks throughout.

// Item kinds.
const (
	KindChain       = "chain"
	KindBundle      = "bundle"
	KindEvents      = "events"
	KindUnsegmented = "unsegmented"
	KindCacheShaped = "cache-under-evidence"
	KindSourceCopy  = "source-copy-under-evidence"
)

// Disposal steps and rules.
const (
	StepCompact = "compact"
	StepRemove  = "remove"

	RuleBound         = "bound"
	RuleMachineCap    = "machine-cap"
	RulePerson        = "person"
	RuleMachineRemove = "machine-remove"
)

// Tombstone states.
const (
	StateBegun = "begun"
	StateDone  = "done"
)

// The names a disposal writes.
const (
	CompactTombstoneName = "DISPOSED.json"
	CompactSidecarName   = "DISPOSED.files.jsonl.gz"
	VerdictName          = "VERDICT.txt"
	removedSuffix        = ".disposed.json"
	removedSidecarSuffix = ".disposed.files.jsonl.gz"
	disposingMark        = ".disposing-"
	// TombstoneSchema names the tombstone's format.
	TombstoneSchema = "metasystem.evidence-tombstone/1"
	// ReceiptSchema is the disposal receipt's schema number.
	ReceiptSchema = 3
	// sidecarAbove: a tombstone's file map larger than this is written as a
	// gzipped JSON-lines sidecar the tombstone names (3.12).
	sidecarAbove = 1 << 20
)

// RemovedTombstonePath is a removed item's tombstone beside it.
func RemovedTombstonePath(item string) string { return item + removedSuffix }

// RemovedSidecarPath is a removed item's inventory sidecar beside it.
func RemovedSidecarPath(item string) string { return item + removedSidecarSuffix }

// IsDisposalRecord reports a name that is a tombstone, sidecar or aside
// item of a disposal: never an item of its own.
func IsDisposalRecord(name string) bool {
	return strings.HasSuffix(name, removedSuffix) || strings.HasSuffix(name, removedSidecarSuffix) || strings.Contains(name, disposingMark)
}

// InventoryFile is one file of an item's inventory.
type InventoryFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	Link   string `json:"link,omitempty"`
	Kept   bool   `json:"kept,omitempty"`
	// Original is a distilled file's logical original, answered through its
	// recipe line (3.12: a pointer to the pre-distillation path answers).
	Original *RecipeLine `json:"original,omitempty"`
	// History, in a sidecar, is 0 for the disposal's own walk and n for the
	// n-th entry of the tombstone's history.
	History int `json:"history,omitempty"`
}

// HistoryEntry is an earlier tombstone of the same item, carried whole.
type HistoryEntry struct {
	Receipt   string          `json:"receipt"`
	Tombstone json.RawMessage `json:"tombstone"`
}

// ExportRef is where an item was exported before its removal.
type ExportRef struct {
	Dir           string    `json:"dir"`
	Archive       string    `json:"archive"`
	ArchiveSHA256 string    `json:"archiveSha256"`
	Manifest      string    `json:"manifest"`
	VerifiedAt    time.Time `json:"verifiedAt"`
}

// Tombstone is the durable disposal state of one item.
type Tombstone struct {
	Schema          string            `json:"schema"`
	Item            string            `json:"item"`
	Kind            string            `json:"kind"`
	Segment         string            `json:"segment,omitempty"`
	Checkout        string            `json:"checkout,omitempty"`
	Files           []InventoryFile   `json:"files,omitempty"`
	Sidecar         string            `json:"sidecar,omitempty"`
	History         []HistoryEntry    `json:"history"`
	ManifestSHA256  string            `json:"manifestSha256,omitempty"`
	InventoryDigest string            `json:"inventoryDigest"`
	Step            string            `json:"step"`
	Rule            string            `json:"rule"`
	By              string            `json:"by"`
	At              time.Time         `json:"at"`
	Receipt         string            `json:"receipt"`
	LedgerTip       string            `json:"ledgerTip,omitempty"`
	LedgerIdentity  string            `json:"ledgerIdentity,omitempty"`
	BytesBefore     int64             `json:"bytesBefore"`
	BytesAfter      int64             `json:"bytesAfter"`
	State           string            `json:"state"`
	Overrides       []string          `json:"overrides,omitempty"`
	Export          *ExportRef        `json:"export,omitempty"`
	Settings        map[string]string `json:"settings,omitempty"`
	// Disposing is a removal's aside name while it is uncommitted.
	Disposing string `json:"disposing,omitempty"`
	// VerdictWritten says the compaction wrote VERDICT.txt (a rollback
	// removes it).
	VerdictWritten bool `json:"verdictWritten,omitempty"`
}

// DisposalReceipt is one line of a segment's disposals ledger.
type DisposalReceipt struct {
	Schema             int               `json:"schema"`
	ID                 string            `json:"id"`
	At                 time.Time         `json:"at"`
	Segment            string            `json:"segment,omitempty"`
	Checkout           string            `json:"checkout,omitempty"`
	Kind               string            `json:"kind"`
	Item               string            `json:"item"`
	Step               string            `json:"step"`
	Rule               string            `json:"rule"`
	By                 string            `json:"by"`
	LedgerTip          string            `json:"ledgerTip,omitempty"`
	LedgerIdentity     string            `json:"ledgerIdentity,omitempty"`
	InventoryDigest    string            `json:"inventoryDigest"`
	Settings           map[string]string `json:"settings,omitempty"`
	SegmentBytesBefore int64             `json:"segmentBytesBefore,omitempty"`
	BlobChargeBytes    int64             `json:"blobChargeBytes,omitempty"`
	ItemBytesBefore    int64             `json:"itemBytesBefore"`
	ItemBytesAfter     int64             `json:"itemBytesAfter"`
	ManifestSHA256     string            `json:"manifestSha256,omitempty"`
	EndedAt            string            `json:"endedAt,omitempty"`
	Goal               string            `json:"goal,omitempty"`
	GoalState          string            `json:"goalState,omitempty"`
	UncoveredReceipts  int               `json:"uncoveredReceipts"`
	Citations          int               `json:"citations"`
	Dropped            int               `json:"dropped"`
	Tombstone          string            `json:"tombstone"`
	Plan               string            `json:"plan,omitempty"`
	Reason             string            `json:"reason,omitempty"`
	Overrides          []string          `json:"overrides,omitempty"`
	Export             *ExportRef        `json:"export,omitempty"`
}

// Inventory walks one item (a directory, or a single file such as an events
// archive): every regular file's path, size and sha256, a symlink with its
// target, a nested .git as its files, and each DISTILLED.txt line's logical
// original. kept marks the members a compaction keeps.
func Inventory(ctx context.Context, item string, kept func(rel string) bool) ([]InventoryFile, error) {
	info, err := os.Lstat(item)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is neither a directory nor a regular file", item)
		}
		digest, size, err := FileDigest(ctx, item)
		if err != nil {
			return nil, err
		}
		return []InventoryFile{{Path: filepath.Base(item), Size: size, SHA256: digest}}, nil
	}
	var files []InventoryFile
	err = filepath.WalkDir(item, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if full == item || entry.IsDir() {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(full, item+string(filepath.Separator)))
		file := InventoryFile{Path: rel}
		switch {
		case entry.Type()&os.ModeSymlink != 0:
			target, err := os.Readlink(full)
			if err != nil {
				return err
			}
			file.Link = target
		case entry.Type().IsRegular():
			digest, size, err := FileDigest(ctx, full)
			if err != nil {
				return err
			}
			file.Size, file.SHA256 = size, digest
		default:
			return nil
		}
		file.Kept = kept != nil && kept(rel)
		files = append(files, file)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, lines, present, err := ReadDistilled(item); err == nil && present {
		for index := range lines {
			line := lines[index]
			if line.Kind == RecipeCollision {
				continue
			}
			files = append(files, InventoryFile{Path: line.Path, Size: line.Size, SHA256: line.SHA256, Original: &line})
		}
	}
	sort.SliceStable(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// InventoryDigest is the sha256 over an item's sorted physical inventory
// lines (path, size, sha256; a symlink with its target): the export's key.
func InventoryDigest(files []InventoryFile) string {
	var lines []string
	for _, file := range files {
		if file.Original != nil || file.History != 0 {
			continue
		}
		if file.Link != "" {
			lines = append(lines, fmt.Sprintf("%s\tlink\t%s", file.Path, file.Link))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s\t%d\t%s", file.Path, file.Size, file.SHA256))
	}
	sort.Strings(lines)
	hash := sha256.New()
	for _, line := range lines {
		hash.Write([]byte(line + "\n"))
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// DisposalStep is one disposal's inputs. The caller holds the bound lock
// and, for a chain, every job lifecycle lock of the chain.
type DisposalStep struct {
	// Item is the item's absolute path.
	Item string
	// Receipt carries the judgement's fields; the step fills ID, At,
	// InventoryDigest, the byte counts, Dropped and Tombstone.
	Receipt DisposalReceipt
	// Ledger is the segment's disposals ledger.
	Ledger string
	// Kept is a compaction's kept set; nil is a removal.
	Kept func(rel string) bool
	// Verdict is a compaction's VERDICT.txt.
	Verdict []byte
	// Commit runs immediately before the receipt append (the commit point);
	// an error rolls the step back with no receipt and is reported.
	Commit func() error
	Stage  string
	Sync   Syncer
	// interrupt, in tests, stops the step after the named point.
	interrupt func(point string) bool
}

// DisposalResult is what a disposal did.
type DisposalResult struct {
	Receipt    DisposalReceipt `json:"receipt"`
	Tombstone  string          `json:"tombstone"`
	Already    bool            `json:"already,omitempty"`
	RolledBack string          `json:"rolledBack,omitempty"`
}

// ErrDisposalOpen is an item whose earlier disposal has a begun tombstone:
// it is recovered first.
var ErrDisposalOpen = errors.New("an earlier disposal of this item is not finished; it is recovered first")

func (s DisposalStep) stop(point string) bool { return s.interrupt != nil && s.interrupt(point) }

// compactTombstonePath is a compacted item's own tombstone.
func compactTombstonePath(item string) string { return filepath.Join(item, CompactTombstoneName) }

// ReadTombstone reads a tombstone file.
func ReadTombstone(path string) (Tombstone, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Tombstone{}, err
	}
	var tombstone Tombstone
	if err := json.Unmarshal(data, &tombstone); err != nil || tombstone.Schema != TombstoneSchema {
		if err == nil {
			err = errors.New("not a tombstone")
		}
		return Tombstone{}, fmt.Errorf("%s is unreadable: %w", path, err)
	}
	return tombstone, nil
}

// ItemTombstones reads an item's compaction and removal tombstones; an
// absent one is nil.
func ItemTombstones(item string) (compacted, removed *Tombstone, err error) {
	if tombstone, readErr := ReadTombstone(compactTombstonePath(item)); readErr == nil {
		compacted = &tombstone
	} else if !errors.Is(readErr, os.ErrNotExist) && !isNotDir(readErr) {
		return nil, nil, readErr
	}
	if tombstone, readErr := ReadTombstone(RemovedTombstonePath(item)); readErr == nil {
		removed = &tombstone
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return nil, nil, readErr
	}
	return compacted, removed, nil
}

func isNotDir(err error) bool {
	var pathErr *fs.PathError
	return errors.As(err, &pathErr) && strings.Contains(pathErr.Err.Error(), "not a directory")
}

// Dispose runs one disposal step: compaction when step.Kept is set, else
// removal. A repeat of a finished disposal writes nothing (R-129).
func Dispose(ctx context.Context, step DisposalStep) (DisposalResult, error) {
	compacted, removed, err := ItemTombstones(step.Item)
	if err != nil {
		return DisposalResult{}, err
	}
	switch {
	case removed != nil && removed.State == StateDone:
		return DisposalResult{Already: true, Tombstone: RemovedTombstonePath(step.Item)}, nil
	case removed != nil || compacted != nil && compacted.State == StateBegun:
		return DisposalResult{}, ErrDisposalOpen
	case step.Kept != nil && compacted != nil:
		return DisposalResult{Already: true, Tombstone: compactTombstonePath(step.Item)}, nil
	}
	if _, err := os.Lstat(step.Item); err != nil {
		return DisposalResult{}, err
	}
	if step.Kept != nil {
		return compact(ctx, step)
	}
	return remove(ctx, step, compacted)
}

// keptAlways are the members every compaction keeps.
func keptAlways(rel string) bool {
	switch rel {
	case CompactTombstoneName, CompactSidecarName, VerdictName:
		return true
	}
	return false
}

func compact(ctx context.Context, step DisposalStep) (DisposalResult, error) {
	kept := func(rel string) bool { return keptAlways(rel) || step.Kept(rel) }
	files, err := Inventory(ctx, step.Item, kept)
	if err != nil {
		return DisposalResult{}, err
	}
	before, _, _ := Measure(ctx, step.Item)
	tombstone := step.newTombstone(files, before, StepCompact)
	if !hasFile(files, VerdictName) && len(step.Verdict) > 0 {
		if err := step.Sync.WriteDurable(filepath.Join(step.Item, VerdictName), step.Verdict, step.Stage); err != nil {
			return DisposalResult{}, err
		}
		tombstone.VerdictWritten = true
	}
	path := compactTombstonePath(step.Item)
	if err := step.writeTombstone(path, filepath.Join(step.Item, CompactSidecarName), &tombstone, files, nil); err != nil {
		return DisposalResult{}, step.rollback(ctx, tombstone, path, err)
	}
	if step.stop("tombstone") {
		return DisposalResult{}, errInterrupted
	}
	return step.commitAndFinish(ctx, tombstone, path)
}

func hasFile(files []InventoryFile, rel string) bool {
	for _, file := range files {
		if file.Original == nil && file.Path == rel {
			return true
		}
	}
	return false
}

func remove(ctx context.Context, step DisposalStep, compacted *Tombstone) (DisposalResult, error) {
	files, err := Inventory(ctx, step.Item, nil)
	if err != nil {
		return DisposalResult{}, err
	}
	before, _, _ := Measure(ctx, step.Item)
	tombstone := step.newTombstone(files, before, StepRemove)
	var history []InventoryFile
	if compacted != nil {
		raw, err := os.ReadFile(compactTombstonePath(step.Item))
		if err != nil {
			return DisposalResult{}, err
		}
		tombstone.History = append(tombstone.History, HistoryEntry{Receipt: compacted.Receipt, Tombstone: raw})
		// The compaction's sidecar is copied into the removal's own, never
		// moved: a rollback leaves the compacted item exactly as it was.
		if compacted.Sidecar != "" {
			if history, err = readSidecar(filepath.Join(step.Item, compacted.Sidecar)); err != nil {
				return DisposalResult{}, err
			}
			for index := range history {
				history[index].History = 1
			}
		}
	}
	tombstone.Disposing = filepath.Base(step.Item) + disposingMark + step.Stage
	path := RemovedTombstonePath(step.Item)
	if err := step.writeTombstone(path, RemovedSidecarPath(step.Item), &tombstone, files, history); err != nil {
		return DisposalResult{}, step.rollback(ctx, tombstone, path, err)
	}
	if step.stop("tombstone") {
		return DisposalResult{}, errInterrupted
	}
	aside := filepath.Join(filepath.Dir(step.Item), tombstone.Disposing)
	if err := os.Rename(step.Item, aside); err != nil {
		return DisposalResult{}, step.rollback(ctx, tombstone, path, err)
	}
	if err := step.Sync.SyncDir(filepath.Dir(step.Item)); err != nil {
		return DisposalResult{}, step.rollback(ctx, tombstone, path, err)
	}
	if step.stop("aside") {
		return DisposalResult{}, errInterrupted
	}
	return step.commitAndFinish(ctx, tombstone, path)
}

func (s DisposalStep) newTombstone(files []InventoryFile, before int64, kind string) Tombstone {
	receipt := s.Receipt
	tombstone := Tombstone{Schema: TombstoneSchema, Item: filepath.Base(s.Item), Kind: receipt.Kind, Segment: receipt.Segment,
		Checkout: receipt.Checkout, History: []HistoryEntry{}, InventoryDigest: InventoryDigest(files), Step: kind, Rule: receipt.Rule,
		By: receipt.By, At: receipt.At.UTC(), Receipt: receipt.ID, LedgerTip: receipt.LedgerTip, LedgerIdentity: receipt.LedgerIdentity,
		BytesBefore: before, State: StateBegun, Overrides: receipt.Overrides, Export: receipt.Export, Settings: receipt.Settings}
	if data, err := os.ReadFile(filepath.Join(s.Item, "manifest.json")); err == nil {
		sum := sha256.Sum256(data)
		tombstone.ManifestSHA256 = hex.EncodeToString(sum[:])
	}
	return tombstone
}

// writeTombstone writes the inventory (inline, or as a sidecar when large
// or when an earlier sidecar is carried) and the tombstone, each durable.
func (s DisposalStep) writeTombstone(path, sidecar string, tombstone *Tombstone, files, history []InventoryFile) error {
	inline, err := json.Marshal(files)
	if err != nil {
		return err
	}
	if len(inline) > sidecarAbove || len(history) > 0 {
		var buffer bytes.Buffer
		writer := gzip.NewWriter(&buffer)
		encoder := json.NewEncoder(writer)
		for _, file := range append(append([]InventoryFile(nil), files...), history...) {
			if err := encoder.Encode(file); err != nil {
				return err
			}
		}
		if err := writer.Close(); err != nil {
			return err
		}
		if err := s.Sync.WriteDurable(sidecar, buffer.Bytes(), s.Stage); err != nil {
			return err
		}
		tombstone.Sidecar = filepath.Base(sidecar)
	} else {
		tombstone.Files = files
	}
	return s.writeTombstoneFile(path, *tombstone)
}

func (s DisposalStep) writeTombstoneFile(path string, tombstone Tombstone) error {
	data, err := json.MarshalIndent(tombstone, "", "  ")
	if err != nil {
		return err
	}
	return s.Sync.WriteDurable(path, append(data, '\n'), s.Stage)
}

func readSidecar(path string) ([]InventoryFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var files []InventoryFile
	decoder := json.NewDecoder(reader)
	for {
		var entry InventoryFile
		if err := decoder.Decode(&entry); errors.Is(err, io.EOF) {
			return files, nil
		} else if err != nil {
			return nil, fmt.Errorf("sidecar %s is unreadable: %w", path, err)
		}
		files = append(files, entry)
	}
}

// TombstoneFiles is a tombstone's whole inventory: inline, or its sidecar
// read beside the tombstone at path.
func TombstoneFiles(path string, tombstone Tombstone) ([]InventoryFile, error) {
	if tombstone.Sidecar == "" {
		return tombstone.Files, nil
	}
	return readSidecar(filepath.Join(filepath.Dir(path), tombstone.Sidecar))
}

// commitAndFinish is steps (3) to (5): the commit hook, the receipt line
// appended and synced (the commit point), the dropped members removed, the
// tombstone marked done.
func (s DisposalStep) commitAndFinish(ctx context.Context, tombstone Tombstone, path string) (DisposalResult, error) {
	if s.Commit != nil {
		if err := s.Commit(); err != nil {
			rollbackErr := s.rollback(ctx, tombstone, path, nil)
			return DisposalResult{RolledBack: err.Error()}, rollbackErr
		}
	}
	files, err := TombstoneFiles(path, tombstone)
	if err != nil {
		return DisposalResult{}, err
	}
	receipt := s.Receipt
	receipt.Schema, receipt.At, receipt.ID = ReceiptSchema, receipt.At.UTC(), tombstone.Receipt
	receipt.Step, receipt.Kind, receipt.InventoryDigest = tombstone.Step, tombstone.Kind, tombstone.InventoryDigest
	receipt.ItemBytesBefore, receipt.ManifestSHA256 = tombstone.BytesBefore, tombstone.ManifestSHA256
	receipt.Tombstone, receipt.Dropped = path, dropped(files, tombstone.Step)
	if err := AppendReceipt(s.Ledger, receipt, s.Sync); err != nil {
		return DisposalResult{}, err
	}
	if s.stop("receipt") {
		return DisposalResult{}, errInterrupted
	}
	return finish(ctx, s.Item, path, tombstone, receipt, s.Sync, s.Stage)
}

func dropped(files []InventoryFile, step string) int {
	count := 0
	for _, file := range files {
		if file.Original == nil && file.History == 0 && (step == StepRemove || !file.Kept) {
			count++
		}
	}
	return count
}

// finish is steps (4) and (5), for a committed disposal: idempotent, so a
// recovery after the receipt runs it again.
func finish(ctx context.Context, item, path string, tombstone Tombstone, receipt DisposalReceipt, sync Syncer, stage string) (DisposalResult, error) {
	if tombstone.Step == StepRemove {
		if err := RemoveTree(ctx, filepath.Join(filepath.Dir(item), tombstone.Disposing)); err != nil {
			return DisposalResult{}, err
		}
		if err := RemoveTree(ctx, item); err != nil { // cut short before the rename
			return DisposalResult{}, err
		}
	} else {
		files, err := TombstoneFiles(path, tombstone)
		if err != nil {
			return DisposalResult{}, err
		}
		keep := map[string]bool{}
		for _, file := range files {
			if file.Kept && file.Original == nil {
				keep[file.Path] = true
			}
		}
		if err := removeUnkept(ctx, item, keep); err != nil {
			return DisposalResult{}, err
		}
	}
	after, _, _ := Measure(ctx, item)
	tombstone.State, tombstone.BytesAfter = StateDone, after
	step := DisposalStep{Sync: sync, Stage: stage}
	if err := step.writeTombstoneFile(path, tombstone); err != nil {
		return DisposalResult{}, err
	}
	receipt.ItemBytesAfter = after
	return DisposalResult{Receipt: receipt, Tombstone: path}, nil
}

// removeUnkept removes every member of a compacted item that is neither in
// its kept set nor one of the compaction's own files, deepest first, and
// the directories that leaves empty.
func removeUnkept(ctx context.Context, item string, keep map[string]bool) error {
	var members, directories []string
	err := filepath.WalkDir(item, func(full string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if full == item {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(full, item+string(filepath.Separator)))
		if entry.IsDir() {
			directories = append(directories, full)
			return nil
		}
		if !keep[rel] && !keptAlways(rel) && !IsPartial(entry.Name()) {
			members = append(members, full)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, member := range members {
		if err := removeEntry(ctx, member, nil); err != nil {
			return err
		}
	}
	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, directory := range directories {
		_ = os.Remove(directory) // only an emptied directory goes
	}
	return nil
}

// rollback undoes an uncommitted disposal: the item renamed back, the
// tombstone, its sidecar and a written verdict removed. cause, when set, is
// returned joined with any rollback failure.
func (s DisposalStep) rollback(ctx context.Context, tombstone Tombstone, path string, cause error) error {
	return errors.Join(cause, rollbackTombstone(ctx, s.Item, path, tombstone))
}

func rollbackTombstone(ctx context.Context, item, path string, tombstone Tombstone) error {
	var errs []error
	if tombstone.Step == StepRemove && tombstone.Disposing != "" {
		aside := filepath.Join(filepath.Dir(item), tombstone.Disposing)
		if _, err := os.Lstat(aside); err == nil {
			if _, err := os.Lstat(item); errors.Is(err, os.ErrNotExist) {
				errs = append(errs, os.Rename(aside, item))
			}
		}
	}
	if tombstone.VerdictWritten {
		errs = append(errs, ignoreMissing(os.Remove(filepath.Join(item, VerdictName))))
	}
	if tombstone.Sidecar != "" {
		errs = append(errs, ignoreMissing(os.Remove(filepath.Join(filepath.Dir(path), tombstone.Sidecar))))
	}
	errs = append(errs, ignoreMissing(os.Remove(path)))
	errs = append(errs, syncDirectory(filepath.Dir(item)))
	_ = ctx
	return errors.Join(errs...)
}

func ignoreMissing(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// AppendReceipt appends one receipt line to a disposals ledger and syncs it;
// a ledger created by this append has its directory synced too.
func AppendReceipt(ledger string, receipt DisposalReceipt, sync Syncer) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	directory := filepath.Dir(ledger)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	_, statErr := os.Lstat(ledger)
	created := errors.Is(statErr, os.ErrNotExist)
	file, err := os.OpenFile(ledger, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	if created {
		return errors.Join(sync.SyncDir(directory), sync.SyncDir(filepath.Dir(directory)))
	}
	return nil
}

// ReadReceipts reads a disposals ledger; absent is empty.
func ReadReceipts(ledger string) ([]DisposalReceipt, error) {
	data, err := os.ReadFile(ledger)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var receipts []DisposalReceipt
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var receipt DisposalReceipt
		if err := json.Unmarshal([]byte(text), &receipt); err != nil {
			return nil, fmt.Errorf("%s has an unreadable line: %w", ledger, err)
		}
		receipts = append(receipts, receipt)
	}
	return receipts, scanner.Err()
}

// ReceiptCommitted reports whether a receipt id is in the ledger.
func ReceiptCommitted(ledger, id string) (bool, error) {
	receipts, err := ReadReceipts(ledger)
	if err != nil {
		return false, err
	}
	for _, receipt := range receipts {
		if receipt.ID == id {
			return true, nil
		}
	}
	return false, nil
}

// OpenDisposal finds an item's begun tombstone, if any.
func OpenDisposal(item string) (path string, tombstone Tombstone, open bool, err error) {
	compacted, removed, err := ItemTombstones(item)
	if err != nil {
		return "", Tombstone{}, false, err
	}
	if removed != nil && removed.State == StateBegun {
		return RemovedTombstonePath(item), *removed, true, nil
	}
	if compacted != nil && compacted.State == StateBegun {
		return compactTombstonePath(item), *compacted, true, nil
	}
	return "", Tombstone{}, false, nil
}

// Recovery is what a restart does with a begun tombstone whose receipt is
// absent: Continue with a commit hook, or roll back naming why.
type Recovery struct {
	Continue bool
	Commit   func() error
	Reason   string
}

// RecoverDisposal settles an item's begun disposal under the bound lock
// (3.12 "Commit point and recovery"): with its receipt in the ledger it is
// committed and is finished, never re-judged; without it, rejudge decides
// (after a fresh ledger observation and, for an export, a re-verification
// of the archive on disk) whether to continue at the commit point or to roll
// back, leaving the item as it was.
func RecoverDisposal(ctx context.Context, item, ledger string, receipt DisposalReceipt, sync Syncer, stage string,
	rejudge func(Tombstone) Recovery) (DisposalResult, error) {
	path, tombstone, open, err := OpenDisposal(item)
	if err != nil || !open {
		return DisposalResult{}, err
	}
	committed, err := ReceiptCommitted(ledger, tombstone.Receipt)
	if err != nil {
		return DisposalResult{}, err
	}
	if committed {
		receipt.ID = tombstone.Receipt
		return finish(ctx, item, path, tombstone, receipt, sync, stage)
	}
	decision := rejudge(tombstone)
	if !decision.Continue {
		return DisposalResult{RolledBack: decision.Reason}, rollbackTombstone(ctx, item, path, tombstone)
	}
	if tombstone.Step == StepRemove {
		aside := filepath.Join(filepath.Dir(item), tombstone.Disposing)
		if _, err := os.Lstat(item); err == nil {
			if err := os.Rename(item, aside); err != nil {
				return DisposalResult{}, err
			}
			if err := sync.SyncDir(filepath.Dir(item)); err != nil {
				return DisposalResult{}, err
			}
		}
	}
	receipt.ID, receipt.At = tombstone.Receipt, tombstone.At
	step := DisposalStep{Item: item, Receipt: receipt, Ledger: ledger, Commit: decision.Commit, Stage: stage, Sync: sync}
	return step.commitAndFinish(ctx, tombstone, path)
}
