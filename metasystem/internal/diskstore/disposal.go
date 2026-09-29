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
	"golang.org/x/sys/unix"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// A person's removal of evidence (3.12 "Inventory before the first
// unlink", "The tombstone and the pointer", "Commit point", "The receipt";
// R20; Round B2-3: machinery never compacts or removes). It first walks the
// item and writes every file's path, size and sha256 into a tombstone that
// is durable (file and directory synced) before anything else; it then
// renames the item aside; the receipt line appended and synced to
// <the evidence root>/disposals/<segment>.jsonl is the commit point; only then is
// the set-aside copy removed and the tombstone marked done. An interrupted
// removal is only ever rolled back before its commit point, never
// continued; after it, only the recorded set-aside copy is removed. The
// caller holds the bound lock and, for a chain, its job lifecycle locks.

// Item kinds.
const (
	KindChain       = "chain"
	KindBundle      = "bundle"
	KindEvents      = "events"
	KindCacheShaped = "cache-under-evidence"
	KindSourceCopy  = "source-copy-under-evidence"
)

// Disposal steps and rules.
const (
	StepRemove = "remove"

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
	// InventoryDigest is the inventory the export holds; a removal goes
	// only when it equals the planned inventory and the one removed.
	InventoryDigest string `json:"inventoryDigest,omitempty"`
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
	// Plan is a person's plan the disposal executes: the command that
	// finishes or rolls back an interrupted one names it.
	Plan string `json:"plan,omitempty"`
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
// original.
func Inventory(ctx context.Context, item string) ([]InventoryFile, error) {
	return inventory(ctx, item, false)
}

// InventoryComplete is Inventory that fails closed (Round B2-2, R3): an
// unreadable DISTILLED.txt is an error, so an item whose recipes cannot be
// read is never exported as if it had none.
func InventoryComplete(ctx context.Context, item string) ([]InventoryFile, error) {
	return inventory(ctx, item, true)
}

func inventory(ctx context.Context, item string, strict bool) ([]InventoryFile, error) {
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
		files = append(files, file)
		return nil
	})
	if err != nil {
		return nil, err
	}
	_, lines, present, err := ReadDistilled(item)
	if err != nil && strict {
		return nil, fmt.Errorf("its recipes cannot be read: %w", err)
	}
	if err == nil && present {
		for index := range lines {
			line := lines[index]
			if !line.Restores() {
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

// DisposalStep is one removal's inputs. The caller holds the bound lock
// and, for a chain, every job lifecycle lock of the chain.
type DisposalStep struct {
	// Item is the item's absolute path.
	Item string
	// Receipt carries the judgement's fields and a minted ID; the step
	// fills InventoryDigest, the byte counts, Dropped and Tombstone.
	Receipt DisposalReceipt
	// Ledger is the segment's disposals ledger.
	Ledger string
	// PlannedDigest is the inventory digest the person previewed; with an
	// export, the export must hold exactly it and what is removed.
	PlannedDigest string
	// Commit runs immediately before the receipt append (the commit point);
	// an error rolls the step back with no receipt and is reported.
	Commit func() error
	Stage  string
	Sync   Syncer
	// interrupt, in tests, stops the step after the named point.
	interrupt func(point string) bool
}

// DisposalResult is what a removal did.
type DisposalResult struct {
	Receipt    DisposalReceipt `json:"receipt"`
	Tombstone  string          `json:"tombstone"`
	Already    bool            `json:"already,omitempty"`
	RolledBack string          `json:"rolledBack,omitempty"`
}

// ErrDisposalOpen is an item whose earlier removal has a begun tombstone:
// it is settled first.
var ErrDisposalOpen = errors.New("an earlier removal of this item is not settled; it is settled first")

func (s DisposalStep) stop(point string) bool { return s.interrupt != nil && s.interrupt(point) }

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
	if !validReceiptID(tombstone.Receipt) {
		return Tombstone{}, fmt.Errorf("%s names no receipt id; it is not a disposal this engine can judge, a person decides", path)
	}
	return tombstone, nil
}

// Dispose removes one item. A repeat of a finished removal writes nothing
// (R-129).
func Dispose(ctx context.Context, step DisposalStep) (DisposalResult, error) {
	if !validReceiptID(step.Receipt.ID) {
		return DisposalResult{}, errors.New("a removal needs a minted receipt id (NewReceiptID); nothing was done")
	}
	removed, err := ReadTombstone(RemovedTombstonePath(step.Item))
	switch {
	case err == nil && removed.State == StateDone:
		return DisposalResult{Already: true, Tombstone: RemovedTombstonePath(step.Item)}, nil
	case err == nil:
		return DisposalResult{}, ErrDisposalOpen
	case !errors.Is(err, os.ErrNotExist):
		return DisposalResult{}, err
	}
	if _, err := os.Lstat(step.Item); err != nil {
		return DisposalResult{}, err
	}
	files, err := Inventory(ctx, step.Item)
	if err != nil {
		return DisposalResult{}, err
	}
	// A removal with an export removes only what the export holds (Round
	// B2-4, F-1b): the export's inventory, the planned one and the one
	// taken now are the same.
	if export := step.Receipt.Export; export != nil {
		now := InventoryDigest(files)
		if export.InventoryDigest == "" || export.InventoryDigest != now || export.InventoryDigest != step.PlannedDigest {
			return DisposalResult{}, fmt.Errorf("the export %s does not hold what would be removed (export %s, planned %s, now %s); nothing was removed, run --preview again",
				export.Archive, short(export.InventoryDigest), short(step.PlannedDigest), short(now))
		}
	}
	before, _, _ := Measure(ctx, step.Item)
	tombstone := step.newTombstone(files, before)
	tombstone.Disposing = filepath.Base(step.Item) + disposingMark + step.Stage
	path := RemovedTombstonePath(step.Item)
	if err := step.writeTombstone(path, RemovedSidecarPath(step.Item), &tombstone, files); err != nil {
		return DisposalResult{}, errors.Join(err, rollbackTombstone(step.Item, path, tombstone))
	}
	if step.stop("tombstone") {
		return DisposalResult{}, errInterrupted
	}
	aside := filepath.Join(filepath.Dir(step.Item), tombstone.Disposing)
	if err := os.Rename(step.Item, aside); err != nil {
		return DisposalResult{}, errors.Join(err, rollbackTombstone(step.Item, path, tombstone))
	}
	if err := step.Sync.SyncDir(filepath.Dir(step.Item)); err != nil {
		return DisposalResult{}, errors.Join(err, rollbackTombstone(step.Item, path, tombstone))
	}
	if step.stop("aside") {
		return DisposalResult{}, errInterrupted
	}
	if step.Commit != nil {
		if err := step.Commit(); err != nil {
			return DisposalResult{RolledBack: err.Error()}, rollbackTombstone(step.Item, path, tombstone)
		}
	}
	receipt := step.Receipt
	receipt.Schema, receipt.At = ReceiptSchema, receipt.At.UTC()
	receipt.Step, receipt.Kind, receipt.InventoryDigest = StepRemove, tombstone.Kind, tombstone.InventoryDigest
	receipt.ItemBytesBefore, receipt.ManifestSHA256 = tombstone.BytesBefore, tombstone.ManifestSHA256
	receipt.Tombstone, receipt.Dropped = path, len(physical(files))
	if err := AppendReceipt(step.Ledger, receipt, step.Sync); err != nil {
		return DisposalResult{}, errors.Join(err, rollbackTombstone(step.Item, path, tombstone))
	}
	if step.stop("receipt") {
		return DisposalResult{}, errInterrupted
	}
	if err := finishAside(ctx, step.Item, path, tombstone, step.Sync, step.Stage); err != nil {
		return DisposalResult{}, err
	}
	return DisposalResult{Receipt: receipt, Tombstone: path}, nil
}

func short(digest string) string {
	if len(digest) > 12 {
		return digest[:12]
	}
	if digest == "" {
		return "none"
	}
	return digest
}

func physical(files []InventoryFile) []InventoryFile {
	var kept []InventoryFile
	for _, file := range files {
		if file.Original == nil && file.History == 0 {
			kept = append(kept, file)
		}
	}
	return kept
}

func (s DisposalStep) newTombstone(files []InventoryFile, before int64) Tombstone {
	receipt := s.Receipt
	tombstone := Tombstone{Schema: TombstoneSchema, Item: filepath.Base(s.Item), Kind: receipt.Kind, Segment: receipt.Segment,
		Checkout: receipt.Checkout, History: []HistoryEntry{}, InventoryDigest: InventoryDigest(files), Step: StepRemove, Rule: receipt.Rule,
		By: receipt.By, At: receipt.At.UTC(), Receipt: receipt.ID, LedgerTip: receipt.LedgerTip, LedgerIdentity: receipt.LedgerIdentity,
		BytesBefore: before, State: StateBegun, Overrides: receipt.Overrides, Export: receipt.Export, Settings: receipt.Settings, Plan: receipt.Plan}
	if data, err := os.ReadFile(filepath.Join(s.Item, "manifest.json")); err == nil {
		sum := sha256.Sum256(data)
		tombstone.ManifestSHA256 = hex.EncodeToString(sum[:])
	}
	return tombstone
}

// writeTombstone writes the inventory (inline, or as a sidecar when large)
// and the tombstone, each durable.
func (s DisposalStep) writeTombstone(path, sidecar string, tombstone *Tombstone, files []InventoryFile) error {
	inline, err := json.Marshal(files)
	if err != nil {
		return err
	}
	if len(inline) > sidecarAbove {
		var buffer bytes.Buffer
		writer := gzip.NewWriter(&buffer)
		encoder := json.NewEncoder(writer)
		for _, file := range files {
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
	return writeTombstoneFile(path, *tombstone, s.Sync, s.Stage)
}

func writeTombstoneFile(path string, tombstone Tombstone, sync Syncer, stage string) error {
	data, err := json.MarshalIndent(tombstone, "", "  ")
	if err != nil {
		return err
	}
	return sync.WriteDurable(path, append(data, '\n'), stage)
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

// asidePath is the set-aside copy a tombstone records, refused when the
// recorded name is empty or is not this item's own set-aside name (Round
// B2-3, N3-2): a finish never removes the item path or a parent.
func asidePath(item string, tombstone Tombstone) (string, error) {
	name := tombstone.Disposing
	if name == "" || filepath.Base(name) != name || !strings.HasPrefix(name, filepath.Base(item)+disposingMark) {
		return "", fmt.Errorf("the tombstone of %s records no valid set-aside name (%q); a person decides", item, name)
	}
	return filepath.Join(filepath.Dir(item), name), nil
}

// finishAside removes the recorded set-aside copy of a committed removal
// and marks the tombstone done; it never touches the item path.
func finishAside(ctx context.Context, item, path string, tombstone Tombstone, sync Syncer, stage string) error {
	aside, err := asidePath(item, tombstone)
	if err != nil {
		return err
	}
	if err := RemoveTree(ctx, aside); err != nil {
		return err
	}
	tombstone.State = StateDone
	return writeTombstoneFile(path, tombstone, sync, stage)
}

// rollbackTombstone undoes an uncommitted removal: the item renamed back
// from its set-aside copy when the item path is free, then the tombstone
// and its sidecar removed. When both the copy and an entry at the item path
// exist, or the rename back fails, the tombstone stays: it is the only
// record of the copy (Round B2-3, N3-3; L-1).
func rollbackTombstone(item, path string, tombstone Tombstone) error {
	if tombstone.Disposing != "" {
		aside, err := asidePath(item, tombstone)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(aside); err == nil {
			if _, err := os.Lstat(item); err == nil {
				return fmt.Errorf("both %s and its set-aside copy %s exist; the tombstone is kept for a person", item, aside)
			}
			if err := os.Rename(aside, item); err != nil {
				return fmt.Errorf("the item could not be renamed back from %s, its tombstone is kept: %w", aside, err)
			}
		}
	}
	if _, err := os.Lstat(item); err != nil {
		return fmt.Errorf("neither %s nor its set-aside copy exists; the tombstone is kept for a person", item)
	}
	var errs []error
	if tombstone.Sidecar != "" {
		errs = append(errs, ignoreMissing(os.Remove(filepath.Join(filepath.Dir(path), tombstone.Sidecar))))
	}
	errs = append(errs, ignoreMissing(os.Remove(path)), syncDirectory(filepath.Dir(item)))
	return errors.Join(errs...)
}

func ignoreMissing(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Settlement is what SettlePersonDisposal did.
type Settlement struct {
	// Finished: the removal was committed and its set-aside copy is gone.
	Finished bool
	// RolledBack: the removal was not committed and the item is back.
	RolledBack bool
	Receipt    string
}

// SettlePersonDisposal settles an item's open removal (Round B2-3, rule
// 2): an uncommitted one is only ever rolled back (the item renamed back
// when its path is free; with both present the tombstone stays and the
// error says so); a committed one only has its recorded set-aside copy
// removed. Nothing is re-judged or continued; the person previews again.
func SettlePersonDisposal(ctx context.Context, item, ledger string, sync Syncer, stage string) (Settlement, error) {
	path, tombstone, open, err := OpenDisposal(item)
	if err != nil || !open {
		return Settlement{}, err
	}
	committed, err := ReceiptCommitted(ledger, tombstone.Receipt, tombstone.Item)
	if err != nil {
		return Settlement{}, err
	}
	if committed {
		return Settlement{Finished: true, Receipt: tombstone.Receipt}, finishAside(ctx, item, path, tombstone, sync, stage)
	}
	return Settlement{RolledBack: true, Receipt: tombstone.Receipt}, rollbackTombstone(item, path, tombstone)
}

// NewReceiptID mints a disposal's receipt id: a ULID, unique per disposal.
func NewReceiptID(now time.Time, entropy io.Reader) (string, error) { return NewID(now, entropy) }

// validReceiptID is a non-empty id of letters, digits, '-' and '_': an
// empty id is invalid everywhere (Round B2, F-1).
func validReceiptID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		if !(r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// AppendReceipt appends one receipt line to a disposals ledger and syncs it;
// a ledger created by this append has its directory synced too (Round
// B2-3, rule 5). A symlinked ledger is refused and never written. A last
// line without its newline that parses as a whole receipt is whole: its
// newline is written and synced first, nothing is truncated. An
// unparseable last line is torn: nothing is appended and the refusal names
// a repair that keeps every parseable receipt. A ledger that cannot be read
// holds.
func AppendReceipt(ledger string, receipt DisposalReceipt, sync Syncer) error {
	data, err := json.Marshal(receipt)
	if err != nil {
		return err
	}
	directory := filepath.Dir(ledger)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	info, statErr := os.Lstat(ledger)
	created := errors.Is(statErr, os.ErrNotExist)
	switch {
	case statErr != nil && !created:
		return statErr
	case statErr == nil && !info.Mode().IsRegular():
		return fmt.Errorf("the disposals ledger %s is not a regular file (a symbolic link or other entry); nothing is appended, a person decides", ledger)
	}
	var prior int64
	var missingNewline bool
	if statErr == nil {
		prior = info.Size()
		tail, err := ledgerTail(ledger, prior)
		if err != nil {
			return err
		}
		if tail.torn {
			return fmt.Errorf("the disposals ledger %s ends in a torn line that is not a whole receipt; nothing is appended until a person repairs it: truncate -s %d %s (keeps every whole receipt)", ledger, tail.keep, ledger)
		}
		missingNewline = tail.missingNewline
	}
	file, err := os.OpenFile(ledger, os.O_CREATE|os.O_APPEND|os.O_WRONLY|unix.O_NOFOLLOW, 0o644)
	if err != nil {
		return err
	}
	if missingNewline {
		if _, err := file.Write([]byte{'\n'}); err != nil {
			return errors.Join(err, file.Close())
		}
		if err := sync.SyncFile(file); err != nil {
			return errors.Join(err, file.Close())
		}
		prior++
	}
	// A failed append is truncated back to the ledger's prior length (its
	// whole receipts), so a full disk never leaves a torn line that poisons
	// every later read (the caller holds the bound lock).
	if _, err := file.Write(append(data, '\n')); err != nil {
		return errors.Join(err, file.Truncate(prior), file.Close())
	}
	if err := sync.SyncFile(file); err != nil {
		return errors.Join(err, file.Truncate(prior), file.Close())
	}
	if err := file.Close(); err != nil {
		return err
	}
	if created {
		return errors.Join(sync.SyncDir(directory), sync.SyncDir(filepath.Dir(directory)))
	}
	return nil
}

type ledgerTailState struct {
	// missingNewline: the last line is a whole receipt without its newline.
	missingNewline bool
	// torn: the last line is not a whole receipt; keep is the length that
	// keeps every whole line before it.
	torn bool
	keep int64
}

// ledgerTail judges a ledger's last line (Round B2-2, R8; Round B2-3, rule
// 5); a read error is returned, so the caller holds.
func ledgerTail(ledger string, size int64) (ledgerTailState, error) {
	if size == 0 {
		return ledgerTailState{}, nil
	}
	data, err := os.ReadFile(ledger)
	if err != nil {
		return ledgerTailState{}, fmt.Errorf("the disposals ledger %s cannot be read, nothing is appended: %w", ledger, err)
	}
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return ledgerTailState{}, nil
	}
	start := bytes.LastIndexByte(data, '\n') + 1
	var last DisposalReceipt
	line := bytes.TrimSuffix(data[start:], []byte{'\r'})
	if json.Unmarshal(line, &last) == nil && last.Schema == ReceiptSchema && validReceiptID(last.ID) {
		return ledgerTailState{missingNewline: true}, nil
	}
	return ledgerTailState{torn: true, keep: int64(start)}, nil
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

// ReceiptCommitted reports whether a receipt with this id, for this item,
// is in the ledger; an empty or malformed id is never committed, and a
// line of another item never commits this one.
func ReceiptCommitted(ledger, id, item string) (bool, error) {
	if !validReceiptID(id) {
		return false, nil
	}
	receipts, err := ReadReceipts(ledger)
	if err != nil {
		return false, err
	}
	for _, receipt := range receipts {
		if receipt.ID == id && receipt.Item == item {
			return true, nil
		}
	}
	return false, nil
}

// OpenRemovals lists the begun removal tombstones in a directory: a
// removal set aside as .disposing-* is no longer listed as an item, and
// its tombstone is how a pass finds it.
func OpenRemovals(directory string) []string {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil
	}
	var items []string
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), removedSuffix)
		if !ok {
			continue
		}
		tombstone, err := ReadTombstone(filepath.Join(directory, entry.Name()))
		if err != nil || tombstone.State == StateBegun {
			items = append(items, filepath.Join(directory, name))
		}
	}
	return items
}

// OpenDisposal finds an item's begun removal tombstone, if any.
func OpenDisposal(item string) (path string, tombstone Tombstone, open bool, err error) {
	tombstone, err = ReadTombstone(RemovedTombstonePath(item))
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", Tombstone{}, false, nil
	case err != nil:
		return "", Tombstone{}, false, err
	}
	return RemovedTombstonePath(item), tombstone, tombstone.State == StateBegun, nil
}
