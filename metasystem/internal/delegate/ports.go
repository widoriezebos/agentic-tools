package delegate

// The read-side ports: what today's CLI runtimes actually provide of
// the seam — usage extraction and result interpretation — registered
// exactly as that, never as a conforming Driver. Each field's
// signature matches its owner function so the port wrapper carries no
// argument-mapping of its own; the differential fixtures prove the
// wiring byte-identical against the direct calls. A field left nil
// means the runtime does not provide that operation; callers refuse
// by name instead of guessing.
//
// Ports key by RUNTIME alone — deliberately narrower than the
// Driver registry's (runtime, transport) identity. A port interprets
// the runtime's native artifacts (a result document, an event log, a
// transcript), and those bytes mean the same thing however the
// session was driven; the devin collector even spans both transports
// in one walk (the acp-outcome channel beside the legacy ones). A
// transport dimension here would force duplicate registrations of
// identical functions.

import (
	"fmt"
	"sort"
	"sync"
)

// CollectInputs re-declares the delegate collector's inputs in seam
// vocabulary, so this package stays a dependency leaf. The owner's
// port maps fields one-for-one.
type CollectInputs struct {
	Root           string
	Job            string
	RoundDir       string
	Workspace      string
	StdoutPath     string
	NamedPath      string
	TranscriptPath string
	ACPOutcomePath string
	RecordPath     string
	Attempt        string
	Session        string
	PresenceOnly   bool
}

// HostCollectInputs mirrors the host-side collector's inputs.
type HostCollectInputs struct {
	Root           string
	TurnRecordPath string
	TurnDir        string
	Workspace      string
	StdoutPath     string
	NamedPath      string
	TranscriptPath string
	RejectDigests  []string
}

// Ports is one runtime's read-side registration. Delegate-side ports
// serve the dispatch adapters; Host-prefixed ports serve the host
// adapters — the same beats on the two sides of the wall.
type Ports struct {
	// The usage beat: one native artifact in, the uniform usage
	// document out (claude reads its result document, codex its event
	// log; a runtime with no native usage ignores the artifact and
	// writes its fixed shape).
	Usage func(artifactPath, outputPath string) error
	// TurnUsage is the delta-accounting variant (devin's ACU
	// accounting across turns).
	TurnUsage func(usagePath, transcriptPath, snapshotPath, cumulativePath, previousPath string, expectPrevious bool) error
	// The result beat: interpretation of native result artifacts.
	ResultField func(resultPath, field string) (value string, present bool, err error)
	Settle      func(transcriptPath, snapshotPath, correlatedSession, roundDir string, requireTranscript bool) (model string, certified bool, err error)
	// Collect walks the delivery channels and returns the verdict
	// document bytes plus its delivered flag (the relay's exit
	// taxonomy needs the flag without re-parsing the document).
	Collect func(in CollectInputs) (verdict []byte, delivered bool, err error)
	// Return writes the runtime's return document (the fake harness's
	// deterministic returns).
	Return func(recordPath, promptPath, outputPath string) error

	// Host-side result beats.
	HostResult     func(providerPath, returnPath, usagePath string) error
	HostReturn     func(rawPath, outputPath string) error
	HostTurnUsage  func(usagePath, transcriptPath, cumulativePath, previousPath string, expectPrevious bool) error
	HostCollect    func(in HostCollectInputs) (verdict []byte, delivered bool, err error)
	HostFakeReturn func(turnPath, statePath, outputPath, behavior, root string) error
	HostFakeResult func(resultPath, session, rawPath, returnPath, outcome string) error
}

var (
	portsMu  sync.Mutex
	portSets = map[string]Ports{}
)

// RegisterPorts registers a runtime's read-side ports under its
// registry key. The two owner packages each register their half —
// the dispatch adapters the delegate-side beats, the host adapters
// the host-side beats — so registration MERGES non-nil fields; a
// field registered twice panics at init exactly like a duplicate
// driver would: broken wiring must not survive to runtime.
func RegisterPorts(key string, p Ports) {
	portsMu.Lock()
	defer portsMu.Unlock()
	if key == "" {
		panic("delegate: port registration needs a key")
	}
	merged := portSets[key]
	merge := func(name string, existing, incoming bool, assign func()) {
		if !incoming {
			return
		}
		if existing {
			panic(fmt.Sprintf("delegate: port %s for %q registered twice", name, key))
		}
		assign()
	}
	merge("Usage", merged.Usage != nil, p.Usage != nil, func() { merged.Usage = p.Usage })
	merge("TurnUsage", merged.TurnUsage != nil, p.TurnUsage != nil, func() { merged.TurnUsage = p.TurnUsage })
	merge("ResultField", merged.ResultField != nil, p.ResultField != nil, func() { merged.ResultField = p.ResultField })
	merge("Settle", merged.Settle != nil, p.Settle != nil, func() { merged.Settle = p.Settle })
	merge("Collect", merged.Collect != nil, p.Collect != nil, func() { merged.Collect = p.Collect })
	merge("Return", merged.Return != nil, p.Return != nil, func() { merged.Return = p.Return })
	merge("HostResult", merged.HostResult != nil, p.HostResult != nil, func() { merged.HostResult = p.HostResult })
	merge("HostReturn", merged.HostReturn != nil, p.HostReturn != nil, func() { merged.HostReturn = p.HostReturn })
	merge("HostTurnUsage", merged.HostTurnUsage != nil, p.HostTurnUsage != nil, func() { merged.HostTurnUsage = p.HostTurnUsage })
	merge("HostCollect", merged.HostCollect != nil, p.HostCollect != nil, func() { merged.HostCollect = p.HostCollect })
	merge("HostFakeReturn", merged.HostFakeReturn != nil, p.HostFakeReturn != nil, func() { merged.HostFakeReturn = p.HostFakeReturn })
	merge("HostFakeResult", merged.HostFakeResult != nil, p.HostFakeResult != nil, func() { merged.HostFakeResult = p.HostFakeResult })
	portSets[key] = merged
}

// PortsFor returns a runtime's registered ports.
func PortsFor(key string) (Ports, error) {
	portsMu.Lock()
	defer portsMu.Unlock()
	p, ok := portSets[key]
	if !ok {
		return Ports{}, fmt.Errorf("delegate: no ports registered for %q (registered: %v)", key, portKeysLocked())
	}
	return p, nil
}

func portKeysLocked() []string {
	keys := make([]string, 0, len(portSets))
	for key := range portSets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
