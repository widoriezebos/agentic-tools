package proofrun

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const inheritedHostResourceFDs = "METASYSTEM_PROOF_RESOURCE_FDS"

var hostAdmissionDirectoryForTest string

// HostResourceLease is the existing host proof admission's active phase.
// File descriptors stay open through the owning child and cleanup; waiting
// for another proof producer never holds one.
type HostResourceLease struct {
	files    []*os.File
	waited   time.Duration
	borrowed bool
}

type hostLeaseRecord struct {
	Schema    int             `json:"schema"`
	Owner     ProcessIdentity `json:"owner"`
	Class     string          `json:"class"`
	Slot      string          `json:"slot,omitempty"`
	Resources []string        `json:"resources"`
	Cleared   bool            `json:"cleared"`
}

const hostLeaseRecordMaxBytes = 16 << 10

func validHostLeaseRecord(path string, record hostLeaseRecord) error {
	name := filepath.Base(path)
	if record.Schema != 1 || record.Owner.Pid <= 0 || !record.Owner.Ref().NativeExact() ||
		(record.Class != "cheap" && record.Class != "heavy") ||
		!strings.HasPrefix(name, "lease-"+record.Class+"-") || len(strings.TrimPrefix(name, "lease-"+record.Class+"-")) != 32 {
		return fmt.Errorf("unreconciled proof resource marker %s has an invalid claim", name)
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(name, "lease-"+record.Class+"-")); err != nil {
		return fmt.Errorf("unreconciled proof resource marker %s has an invalid nonce", name)
	}
	if record.Class == "cheap" && record.Slot != "" || record.Slot != "" && !strings.HasPrefix(record.Slot, "slot-") {
		return fmt.Errorf("unreconciled proof resource marker %s has an invalid slot", name)
	}
	seen := map[string]bool{}
	for _, resource := range record.Resources {
		if seen[resource] || !strings.HasPrefix(resource, "resource-") || len(resource) != len("resource-")+64 {
			return fmt.Errorf("unreconciled proof resource marker %s has an invalid resource", name)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(resource, "resource-")); err != nil {
			return fmt.Errorf("unreconciled proof resource marker %s has an invalid resource", name)
		}
		seen[resource] = true
	}
	return nil
}

func readHostLeaseRecord(file *os.File) (hostLeaseRecord, []byte, error) {
	var record hostLeaseRecord
	if err := validateHostLockFile(file.Name(), file); err != nil {
		return record, nil, err
	}
	data, err := io.ReadAll(io.NewSectionReader(file, 0, hostLeaseRecordMaxBytes+1))
	if err != nil {
		return record, nil, err
	}
	if len(data) > hostLeaseRecordMaxBytes || json.Unmarshal(data, &record) != nil || validHostLeaseRecord(file.Name(), record) != nil {
		return hostLeaseRecord{}, nil, fmt.Errorf("unreconciled proof resource marker %s is unreadable", filepath.Base(file.Name()))
	}
	return record, data, nil
}

func setHostLeaseCleared(file *os.File, data []byte, cleared bool) error {
	// The claim is immutable after acquisition. Only one fixed-width JSON
	// value changes, so a concurrent admission reader never sees a truncated
	// ownership/class/resource claim while the custodian finishes.
	needle := []byte(`"cleared":`)
	index := bytes.Index(data, needle)
	if index < 0 || bytes.Count(data, needle) != 1 {
		return fmt.Errorf("proof resource marker has no unique cleared field")
	}
	index += len(needle)
	if len(data)-index < 5 {
		return fmt.Errorf("proof resource marker has no fixed-width state")
	}
	current := data[index : index+5]
	if !bytes.Equal(current, []byte("true ")) && !bytes.Equal(current, []byte("false")) {
		return fmt.Errorf("proof resource marker state has invalid width")
	}
	state := []byte("false")
	if cleared {
		state = []byte("true ")
	}
	if _, err := file.WriteAt(state, int64(index)); err != nil {
		return err
	}
	return file.Sync()
}

func (lease *HostResourceLease) Files() []*os.File {
	if lease == nil {
		return nil
	}
	return lease.files
}

func (lease *HostResourceLease) Waited() time.Duration {
	if lease == nil {
		return 0
	}
	return lease.waited
}

func (lease *HostResourceLease) Close() error {
	if lease == nil {
		return nil
	}
	var errs []error
	for _, file := range lease.files {
		// Closing, rather than explicitly unlocking, leaves an inherited
		// worker descriptor in custody if the launcher dies unexpectedly.
		errs = append(errs, file.Close())
	}
	lease.files = nil
	return errors.Join(errs...)
}

func hostAdmissionDirectory() (string, error) {
	path := hostAdmissionDirectoryForTest
	if path == "" && os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR") != "" {
		// The request owner checks canonical ancestors before creating the
		// directory, so a temporary symlink cannot redirect fixture admission
		// into production storage.
		return hostAdmissionDirectoryForRequest(os.Getenv("METASYSTEM_PROOF_ADMISSION_FIXTURE_ROOT"),
			os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR"))
	}
	if path == "" {
		account, err := user.Current()
		if err != nil || account.HomeDir == "" {
			return "", fmt.Errorf("locate durable proof admission home: %v", err)
		}
		owner := filepath.Join(account.HomeDir, ".metasystem")
		if err := secureHostAdmissionDirectory(owner, 0o755); err != nil {
			return "", err
		}
		path = filepath.Join(owner, "proof-admission")
	}
	if err := secureHostAdmissionDirectory(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func hostAdmissionDirectoryForRequest(controlRoot, selected string) (string, error) {
	if selected == "" {
		return hostAdmissionDirectory()
	}
	temporaryRoot, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		return "", err
	}
	// macOS exposes the same temporary tree through /var and /private/var.
	// Resolve only an existing ancestor, before creating the requested path.
	ancestor := filepath.Clean(selected)
	var remainder []string
	for {
		if _, statErr := os.Lstat(ancestor); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return "", statErr
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", fmt.Errorf("proof admission test directory has no existing ancestor")
		}
		remainder = append(remainder, filepath.Base(ancestor))
		ancestor = parent
	}
	canonical, err := filepath.EvalSymlinks(ancestor)
	if err != nil {
		return "", err
	}
	for index := len(remainder) - 1; index >= 0; index-- {
		canonical = filepath.Join(canonical, remainder[index])
	}
	relative, err := filepath.Rel(temporaryRoot, canonical)
	if err != nil || !filepath.IsAbs(selected) || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		!fixtureauth.FixtureModeRoot(controlRoot) {
		return "", fmt.Errorf("proof admission test directory requires a temporary path and fake-runtime root")
	}
	if err := secureHostAdmissionDirectory(selected, 0o700); err != nil {
		return "", err
	}
	return selected, nil
}

// FixtureHostAdmissionDirectory exposes the already validated, explicitly
// selected test admission namespace to other host-wide fixture owners. The
// owned checkout must itself authorize fixture mode; an unrelated fake root
// may not move a real owner's global lock. Ordinary processes keep production paths.
func FixtureHostAdmissionDirectory(checkoutRoot string) (string, bool, error) {
	if os.Getenv("METASYSTEM_PROOF_ADMISSION_TEST_DIR") == "" {
		return "", false, nil
	}
	if !fixtureauth.FixtureModeRoot(checkoutRoot) {
		return "", false, fmt.Errorf("proof admission fixture owner checkout must use the fake runtime")
	}
	path, err := hostAdmissionDirectory()
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func secureHostAdmissionDirectory(path string, maximum os.FileMode) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("host proof admission directory is not a directory: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() || info.Mode().Perm()&^maximum != 0 {
		return fmt.Errorf("host proof admission directory has unsafe ownership or permissions")
	}
	return nil
}

func tryHostFile(path string) (*os.File, bool, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := validateHostLockFile(path, file); err != nil {
		file.Close()
		return nil, false, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		if err == syscall.EWOULDBLOCK || err == syscall.EAGAIN {
			return nil, false, nil
		}
		return nil, false, err
	}
	if err := validateHostLockFile(path, file); err != nil {
		file.Close()
		return nil, false, err
	}
	return file, true, nil
}

func validateHostLockFile(path string, file *os.File) error {
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	stat, ok := pathInfo.Sys().(*syscall.Stat_t)
	if !ok || !pathInfo.Mode().IsRegular() || pathInfo.Mode().Perm()&0o077 != 0 ||
		int(stat.Uid) != os.Getuid() || !os.SameFile(pathInfo, fileInfo) {
		return fmt.Errorf("host proof lock file has unsafe ownership, permissions, or inode")
	}
	return nil
}

func proveInheritedHostLock(file *os.File) error {
	probe, acquired, err := tryHostFile(file.Name())
	if probe != nil {
		_ = probe.Close()
	}
	if err != nil || acquired {
		return fmt.Errorf("proof resource lease is not live: %v", err)
	}
	// A peer's flock on the pathname does not prove this open-file
	// description owns it. A separately opened descriptor fails this call.
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("proof resource descriptor does not hold its lock: %w", err)
	}
	return nil
}

func closeHostFiles(files []*os.File) {
	for _, file := range files {
		_ = file.Close()
	}
}

func acquireHostGuard(ctx context.Context, directory string, check func() error) (*os.File, error) {
	path := filepath.Join(directory, "admission.lock")
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if check != nil {
			if err := check(); err != nil {
				return nil, err
			}
		}
		file, acquired, err := tryHostFile(path)
		if err != nil || acquired {
			return file, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func hostResourcePath(directory, resource string) string {
	digest := sha256.Sum256([]byte(resource))
	return filepath.Join(directory, "resource-"+hex.EncodeToString(digest[:]))
}

func activeHostResourceSlots(directory string) (int, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "slot-*"))
	if err != nil {
		return 0, err
	}
	count := 0
	for _, path := range paths {
		file, acquired, err := tryHostFile(path)
		if err != nil {
			return 0, err
		}
		if acquired {
			_ = file.Close()
		} else {
			count++
		}
	}
	dirty, _, err := hostLeaseState(directory)
	return count + dirty, err
}

func hostLeaseState(directory string) (int, map[string]bool, error) {
	return hostLeaseStateWithReclaim(directory, false)
}

// reclaim is permitted only while the caller holds admission.lock. A clean
// marker is discarded after this scanner itself acquires its flock, proving
// no owner or borrower still holds the marker descriptor.
func hostLeaseStateWithReclaim(directory string, reclaim bool) (int, map[string]bool, error) {
	paths, err := filepath.Glob(filepath.Join(directory, "lease-*"))
	if err != nil {
		return 0, nil, err
	}
	dirty, named := 0, map[string]bool{}
	for _, path := range paths {
		// A live marker may be locked by its owner, but its immutable claim
		// still has to be known before admission trusts the slot census.
		reader, err := os.Open(path)
		if err != nil {
			return 0, nil, err
		}
		record, _, err := readHostLeaseRecord(reader)
		_ = reader.Close()
		if err != nil {
			return 0, nil, err
		}
		file, acquired, err := tryHostFile(path)
		if err != nil {
			return 0, nil, err
		}
		if !acquired {
			continue
		}
		// The owner may have marked the marker dirty between the first read
		// and this flock. Decide reclamation from the now-locked inode.
		record, _, err = readHostLeaseRecord(file)
		if err != nil {
			file.Close()
			return 0, nil, err
		}
		if record.Cleared && reclaim {
			if err := os.Remove(path); err != nil {
				file.Close()
				return 0, nil, err
			}
			file.Close()
			continue
		}
		if !record.Cleared {
			if record.Class == "heavy" {
				dirty++
			}
			for _, resource := range record.Resources {
				named[resource] = true
			}
		}
		file.Close()
	}
	return dirty, named, nil
}

// MarkHostResourcesClean is called only after the active command custodian
// proved its ordinary process group empty and declared detached cleanup
// completed. Closing descriptors without this mark leaves a durable dirty
// reservation that fails closed after a crash or unproved cleanup.
func MarkHostResourcesClean(files []*os.File) error { return markHostResources(files, true, nil) }

// MarkHostResourcesDirty precedes every resource-active child birth. A lease
// may be reused for several sequential preparation commands; each command
// must restore crash custody after the previous one was proved clean.
func MarkHostResourcesDirty(files []*os.File) error { return markHostResources(files, false, nil) }

// A custodian may finish the owning launcher's marker after that launcher
// dies, but a nested borrower's custodian never owns the outer reservation.
// The caller must have authenticated parent as its actual launcher at birth.
func MarkHostResourcesCleanCustodian(files []*os.File, parent identity.Ref) error {
	if !parent.NativeExact() {
		return fmt.Errorf("resource custodian parent identity is invalid")
	}
	return markHostResources(files, true, &parent)
}

func markHostResources(files []*os.File, cleared bool, custodianParent *identity.Ref) error {
	for _, file := range files {
		if !strings.HasPrefix(filepath.Base(file.Name()), "lease-") {
			continue
		}
		record, data, err := readHostLeaseRecord(file)
		if err != nil {
			return err
		}
		if err := proveInheritedHostLock(file); err != nil {
			return err
		}
		owner := record.Owner.Ref()
		if custodianParent != nil {
			if owner != *custodianParent {
				return nil
			}
		} else {
			self, err := CurrentProcessIdentity(nil)
			if err != nil {
				return err
			}
			if owner != self.Ref() {
				return nil
			}
		}
		return setHostLeaseCleared(file, data, cleared)
	}
	return fmt.Errorf("proof resource lease has no custody marker")
}

func hostResourceNames(exclusive []string) ([]string, error) {
	resources := append([]string(nil), exclusive...)
	sort.Strings(resources)
	for index, resource := range resources {
		if resource == "" || index > 0 && resource == resources[index-1] {
			return nil, fmt.Errorf("proof exclusive resource names must be nonempty and unique")
		}
	}
	return resources, nil
}

// An authenticated nested launcher can borrow only an inherited, still-locked
// parent lease that already covers every requested resource. A proof locator
// by itself does not represent an active capacity slot.
func borrowHostResources(directory string, parent Attempt, class string, exclusive []string) (*HostResourceLease, error) {
	raw := os.Getenv(inheritedHostResourceFDs)
	if raw == "" {
		if len(exclusive) != 0 {
			return nil, fmt.Errorf("legacy proof parent has no named resource lease")
		}
		rows, known := readProcessRows()
		if !known {
			return nil, fmt.Errorf("legacy proof parent capacity census is unreadable")
		}
		for _, row := range rows {
			if row.pid == parent.Launcher.Pid && row.launcher {
				return &HostResourceLease{borrowed: true}, nil
			}
		}
		return nil, fmt.Errorf("proof parent has no active resource lease or counted legacy launcher")
	}
	var files []*os.File
	covered := map[string]bool{}
	var marker *os.File
	slot := ""
	declaredResources := map[string]bool{}
	for _, entry := range strings.Split(raw, ",") {
		fdText, name, ok := strings.Cut(entry, "@")
		fd, err := strconv.Atoi(fdText)
		if !ok || err != nil || fd < 3 || name == "" || filepath.Base(name) != name ||
			!(strings.HasPrefix(name, "slot-") || strings.HasPrefix(name, "resource-") || strings.HasPrefix(name, "lease-")) || covered[name] {
			closeHostFiles(files)
			return nil, fmt.Errorf("nested proof resource lease manifest is invalid")
		}
		copyFD, err := syscall.Dup(fd)
		if err != nil {
			closeHostFiles(files)
			return nil, fmt.Errorf("nested proof resource descriptor is absent: %w", err)
		}
		path := filepath.Join(directory, name)
		file := os.NewFile(uintptr(copyFD), path)
		if err := validateHostLockFile(path, file); err != nil {
			file.Close()
			closeHostFiles(files)
			return nil, err
		}
		if err := proveInheritedHostLock(file); err != nil {
			file.Close()
			closeHostFiles(files)
			return nil, err
		}
		files = append(files, file)
		covered[name] = true
		switch {
		case strings.HasPrefix(name, "slot-"):
			if slot != "" {
				closeHostFiles(files)
				return nil, fmt.Errorf("nested proof lease has multiple slots")
			}
			slot = name
		case strings.HasPrefix(name, "resource-"):
			declaredResources[name] = true
		case strings.HasPrefix(name, "lease-"):
			if marker != nil {
				closeHostFiles(files)
				return nil, fmt.Errorf("nested proof lease has multiple markers")
			}
			marker = file
		}
	}
	if marker == nil {
		closeHostFiles(files)
		return nil, fmt.Errorf("proof parent lease has no marker")
	}
	record, _, err := readHostLeaseRecord(marker)
	if err != nil {
		closeHostFiles(files)
		return nil, err
	}
	if !lineageContains(int64(os.Getppid()), record.Owner.Ref()) || !lineageContains(record.Owner.Pid, parent.Launcher.Ref()) {
		closeHostFiles(files)
		return nil, fmt.Errorf("proof parent lease owner is outside the authenticated parent")
	}
	if class == "heavy" && record.Class != "heavy" || record.Slot != slot || len(record.Resources) != len(declaredResources) {
		closeHostFiles(files)
		return nil, fmt.Errorf("proof parent lease claim does not match inherited capacity or resources")
	}
	for _, resource := range record.Resources {
		if !declaredResources[resource] {
			closeHostFiles(files)
			return nil, fmt.Errorf("proof parent lease claim omits inherited resource %q", resource)
		}
	}
	for _, resource := range exclusive {
		if !covered[filepath.Base(hostResourcePath(directory, resource))] {
			closeHostFiles(files)
			return nil, fmt.Errorf("proof parent lease does not cover named resource %q", resource)
		}
	}
	return &HostResourceLease{files: files, borrowed: true}, nil
}

// AcquireHostResources uses the same cap and host census as proof attempt
// admission. The only new state is which currently active phases own slots.
// A nested authenticated proof child consumes its parent's reservation.
func AcquireHostResources(ctx context.Context, controlRoot, confPath, class string, exclusive []string) (*HostResourceLease, error) {
	return AcquireHostResourcesWithWaitCheck(ctx, controlRoot, confPath, class, exclusive, nil)
}

type hostResourceWaitObserverKey struct{}

// WithHostResourceWaitObserver reports the first failed resource scan after
// this context is attached. It reports at most once per acquisition, after
// releasing the admission guard and any partial locks; it does not replay
// waits that happened before attachment.
func WithHostResourceWaitObserver(ctx context.Context, observe func()) context.Context {
	if observe == nil {
		return ctx
	}
	return context.WithValue(ctx, hostResourceWaitObserverKey{}, observe)
}

func resourceLegacyLauncherCount(controlRoot string) (int, bool, error) {
	// The existing fixture executable's scripted host count also governs its
	// resource phase, but only in an explicitly selected temporary admission
	// namespace owned by this same fake-runtime checkout. Ordinary engines and
	// production roots still use the real process census.
	if len(commandLoadOptions) != 0 && fixtureauth.FixtureModeRoot(controlRoot) {
		_, selected, err := FixtureHostAdmissionDirectory(controlRoot)
		if err != nil {
			return 0, false, err
		}
		if selected {
			settings := loadSampleSettings{}
			for _, option := range commandLoadOptions {
				option(&settings)
			}
			if settings.fixtureSet {
				_, count, known := testHostLoad(settings.fixtureRaw, time.Now())
				if !known {
					return 0, false, fmt.Errorf("fixture host launcher count is invalid")
				}
				return count, true, nil
			}
		}
	}
	count, known := loadSeams.launchers(int64(os.Getpid()))
	return count, known, nil
}

// AcquireHostResourcesWithWaitCheck lets the caller end a capacity wait when
// its own admission authority changes. The check runs before each retry;
// callers still revalidate authority after acquiring capacity.
func AcquireHostResourcesWithWaitCheck(ctx context.Context, controlRoot, confPath, class string, exclusive []string, check func() error) (*HostResourceLease, error) {
	if class != "cheap" && class != "heavy" {
		return nil, fmt.Errorf("unknown proof resource class %q", class)
	}
	resources, err := hostResourceNames(exclusive)
	if err != nil {
		return nil, err
	}
	directory, err := hostAdmissionDirectory()
	if err != nil {
		return nil, err
	}
	if parentRoot, parentID := os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT"); parentRoot != "" || parentID != "" {
		if parentRoot != controlRoot || parentID == "" {
			return nil, fmt.Errorf("nested proof resource locator does not match its control root")
		}
		parent, err := AuthenticateContext(controlRoot, parentID, int64(os.Getppid()))
		if err != nil {
			return nil, fmt.Errorf("nested proof resource custody: %w", err)
		}
		return borrowHostResources(directory, parent, class, resources)
	}
	cores := hostload.Read(time.Now().UTC()).Cores
	if cores < 1 {
		cores = runtime.NumCPU()
	}
	capacity, err := ResolveAdmissionCap(confPath, cores)
	if err != nil {
		return nil, err
	}
	started := time.Now()
	observedWait := false
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if check != nil {
			if err := check(); err != nil {
				return nil, err
			}
		}
		guard, err := acquireHostGuard(ctx, directory, check)
		if err != nil {
			return nil, err
		}
		_, dirtyNamed, err := hostLeaseStateWithReclaim(directory, true)
		if err != nil {
			guard.Close()
			return nil, err
		}
		files := make([]*os.File, 0, len(resources)+1)
		available := true
		for _, resource := range resources {
			if dirtyNamed[filepath.Base(hostResourcePath(directory, resource))] {
				available = false
			}
		}
		for _, resource := range resources {
			if !available {
				break
			}
			file, acquired, lockErr := tryHostFile(hostResourcePath(directory, resource))
			if lockErr != nil {
				closeHostFiles(files)
				guard.Close()
				return nil, lockErr
			}
			if !acquired {
				available = false
				break
			}
			files = append(files, file)
		}
		if available && class == "heavy" && capacity.Max > 0 {
			legacy, known, censusErr := resourceLegacyLauncherCount(controlRoot)
			active, countErr := activeHostResourceSlots(directory)
			if countErr != nil || censusErr != nil || !known {
				closeHostFiles(files)
				guard.Close()
				if !known && censusErr == nil {
					censusErr = errors.New("process rows are unknown")
				}
				return nil, fmt.Errorf("host proof admission census is unreadable: %w", errors.Join(censusErr, countErr))
			}
			// Compare without adding a fixture-supplied launcher count to
			// active slots: that sum can overflow before the cap check.
			if active >= capacity.Max || legacy >= capacity.Max-active {
				available = false
			} else {
				for index := 0; index < capacity.Max; index++ {
					slot, acquired, lockErr := tryHostFile(filepath.Join(directory, fmt.Sprintf("slot-%02d", index)))
					if lockErr != nil {
						closeHostFiles(files)
						guard.Close()
						return nil, lockErr
					}
					if acquired {
						files = append(files, slot)
						break
					}
				}
				if len(files) == len(resources) {
					available = false
				}
			}
		}
		if available {
			var nonce [16]byte
			if _, err := rand.Read(nonce[:]); err != nil {
				closeHostFiles(files)
				guard.Close()
				return nil, err
			}
			marker, acquired, markerErr := tryHostFile(filepath.Join(directory, "lease-"+class+"-"+hex.EncodeToString(nonce[:])))
			if markerErr != nil || !acquired {
				closeHostFiles(files)
				guard.Close()
				return nil, fmt.Errorf("create proof resource lease marker: %v", markerErr)
			}
			files = append(files, marker)
			claimed := make([]string, len(resources))
			for index, resource := range resources {
				claimed[index] = filepath.Base(hostResourcePath(directory, resource))
			}
			owner, ownerErr := CurrentProcessIdentity(nil)
			if ownerErr != nil {
				closeHostFiles(files)
				guard.Close()
				return nil, ownerErr
			}
			slot := ""
			for _, file := range files {
				if strings.HasPrefix(filepath.Base(file.Name()), "slot-") {
					slot = filepath.Base(file.Name())
					break
				}
			}
			record := hostLeaseRecord{Schema: 1, Owner: owner, Class: class, Slot: slot, Resources: claimed, Cleared: false}
			encoded, encodeErr := json.Marshal(record)
			if encodeErr != nil {
				closeHostFiles(files)
				guard.Close()
				return nil, encodeErr
			}
			if len(encoded) > hostLeaseRecordMaxBytes || validHostLeaseRecord(marker.Name(), record) != nil {
				closeHostFiles(files)
				guard.Close()
				return nil, fmt.Errorf("proof resource lease claim is invalid or too large")
			}
			info, statErr := marker.Stat()
			if statErr != nil || info.Size() != 0 {
				closeHostFiles(files)
				guard.Close()
				return nil, fmt.Errorf("proof resource lease marker already contains a claim: %v", statErr)
			}
			if _, writeErr := marker.Write(encoded); writeErr != nil {
				closeHostFiles(files)
				guard.Close()
				return nil, writeErr
			}
			if syncErr := setHostLeaseCleared(marker, encoded, true); syncErr != nil {
				closeHostFiles(files)
				guard.Close()
				return nil, syncErr
			}
		}
		guard.Close()
		if available {
			return &HostResourceLease{files: files, waited: time.Since(started)}, nil
		}
		closeHostFiles(files)
		if !observedWait {
			if observe, ok := ctx.Value(hostResourceWaitObserverKey{}).(func()); ok {
				observe()
			}
			observedWait = true
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// MarkManagedProofProcess excludes the call's idle and producer-wait phases
// from the legacy argv census. Active phases are counted by locked slots.
func MarkManagedProofProcess() (func(), error) {
	directory, err := hostAdmissionDirectory()
	if err != nil {
		return nil, err
	}
	owner, err := CurrentProcessIdentity(nil)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(directory, fmt.Sprintf("managed-%d.json", owner.Pid))
	data, err := json.Marshal(owner)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}

func managedProofProcess(exact identity.Exact) bool {
	directory, err := hostAdmissionDirectory()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(directory, fmt.Sprintf("managed-%d.json", exact.Pid)))
	if err != nil {
		return false
	}
	var owner ProcessIdentity
	return json.Unmarshal(data, &owner) == nil && identity.Compare(exact, owner.Ref()).Matches
}

type hostResourceContextKey struct{}

func WithHostResourceLease(ctx context.Context, lease *HostResourceLease) context.Context {
	return context.WithValue(ctx, hostResourceContextKey{}, lease)
}

func HostResourceLeaseFromContext(ctx context.Context) *HostResourceLease {
	lease, _ := ctx.Value(hostResourceContextKey{}).(*HostResourceLease)
	return lease
}

func AttachHostResourceLease(ctx context.Context, command *exec.Cmd) {
	lease, _ := ctx.Value(hostResourceContextKey{}).(*HostResourceLease)
	if lease != nil && len(lease.files) != 0 {
		if len(command.ExtraFiles) != 0 {
			return
		}
		command.ExtraFiles = append(command.ExtraFiles, lease.files...)
		command.Env = append(command.Env, HostResourceFDEnvironment(lease.files))
	}
}

// InheritHostResourceLease keeps the resource slot with a directly launched
// native child even if its worker is killed before process cleanup.
func InheritHostResourceLease(command *exec.Cmd) (func(), error) {
	raw := os.Getenv(inheritedHostResourceFDs)
	if raw == "" {
		return func() {}, nil
	}
	var copied []*os.File
	for _, part := range strings.Split(raw, ",") {
		fdText, name, ok := strings.Cut(part, "@")
		fd, err := strconv.Atoi(fdText)
		if !ok || err != nil || fd < 3 || filepath.Base(name) != name {
			closeHostFiles(copied)
			return nil, fmt.Errorf("inherited proof resource manifest is invalid")
		}
		duplicate, err := syscall.Dup(fd)
		if err != nil {
			closeHostFiles(copied)
			return nil, fmt.Errorf("inherited proof resource descriptor is absent: %w", err)
		}
		copied = append(copied, os.NewFile(uintptr(duplicate), name))
	}
	if len(command.ExtraFiles) != 0 {
		closeHostFiles(copied)
		return nil, fmt.Errorf("native command has conflicting resource descriptors")
	}
	command.ExtraFiles = append(command.ExtraFiles, copied...)
	command.Env = append(command.Env, HostResourceFDEnvironment(copied))
	return func() { closeHostFiles(copied) }, nil
}

func HostResourceFDEnvironment(files []*os.File) string {
	ids := make([]string, len(files))
	for index := range files {
		ids[index] = strconv.Itoa(3+index) + "@" + filepath.Base(files[index].Name())
	}
	return inheritedHostResourceFDs + "=" + strings.Join(ids, ",")
}
