package project

// The catalogue: a convention fixed in Go, resolved against the three roots.
//
// It is a table here rather than a file in the repository, because a file
// would be a second source of truth a project would have to maintain, and it
// is not a scan, because a scan would find whatever happens to be lying about.
// Nothing is filtered by ownership; ownership is shown.
//
// The kit's own documents — the paper, docs/design, the architecture map, the
// concepts — are the subject's only when the subject is the MetaSystem. In an
// adopted workspace they belong to the machinery, which is Settings' at gate 7,
// and this section leaves them out.

// The directories a row is read against.
type rootKind int

const (
	rootInstallation rootKind = iota
	rootStateRoot
	rootCheckout
)

// resolvedRoot is one root as this checkout sees it: where it is, what it is
// called relative to the checkout, and whether it lies beneath it at all.
type resolvedRoot struct {
	directory string
	prefix    string
	beneath   bool
}

// What one catalogue row names.
type entryKind int

const (
	// kindFiles names exact paths relative to the root, in the order given.
	kindFiles entryKind = iota
	// kindGlob names one directory and a pattern, listed by name.
	kindGlob
	// kindPaper is the paper's own order: the index, then the chapters.
	kindPaper
	// kindDesigns is every design document beneath a plans directory, one
	// level down included, and never the goal records.
	kindDesigns
	// kindCovenant is the app's declaration at its one home.
	kindCovenant
	// kindPurpose is the one paragraph the rules file declares.
	kindPurpose
)

const (
	paperDir = "docs/paper"
	plansDir = "plans"
)

// sectionRow is one of the master's six subsections.
type sectionRow struct {
	id    string
	title string
	// notProjected is the view this build does not have: the sitting store
	// arrives with the brain process, and nothing here pretends otherwise.
	notProjected bool
	// alwaysNotRecorded marks a subsection with no home at all, whatever else
	// is found near it: no register of open questions exists in this kit.
	alwaysNotRecorded bool
}

var subsections = []sectionRow{
	{id: "intent", title: "Intent"},
	{id: "architecture", title: "Architecture"},
	{id: "designs", title: "Designs"},
	{id: "constraints", title: "Constraints and assurance"},
	{id: "open-questions", title: "Open questions", alwaysNotRecorded: true},
	{id: "sittings", title: "Sittings", notProjected: true},
}

// entryRow is one source: a subsection, the group it lists under, the roots it
// is resolved against, and the modes it applies in.
type entryRow struct {
	subsection string
	groupID    string
	groupTitle string
	roots      []rootKind
	kind       entryKind
	files      []string
	dir        string
	pattern    string
	// selfHostedOnly keeps the kit's own documents out of an adopted
	// workspace, where they are the machinery's and not the subject's.
	selfHostedOnly bool
}

// bothRoots is the pair a project's own material is looked for in: the state
// root, and the checkout when the two are different directories.
var bothRoots = []rootKind{rootStateRoot, rootCheckout}

var entries = []entryRow{
	{subsection: "intent", kind: kindCovenant, roots: bothRoots},
	{subsection: "intent", kind: kindPurpose, roots: []rootKind{rootInstallation},
		files: []string{"docs/project-rules.md"}},

	{subsection: "architecture", groupID: "architecture", kind: kindFiles,
		roots: []rootKind{rootInstallation}, files: []string{"docs/app-doctrine.md"}},
	{subsection: "architecture", groupID: "architecture", kind: kindFiles,
		roots: []rootKind{rootInstallation}, files: []string{"docs/architecture.md", "docs/concepts.md"},
		selfHostedOnly: true},

	{subsection: "designs", groupID: "paper", groupTitle: "The paper", kind: kindPaper,
		roots: []rootKind{rootInstallation}, selfHostedOnly: true},
	{subsection: "designs", groupID: "design-documents", groupTitle: "Design documents", kind: kindGlob,
		roots: []rootKind{rootInstallation}, dir: "docs/design", pattern: "*.md", selfHostedOnly: true},
	{subsection: "designs", groupID: "live-designs", groupTitle: "Live designs", kind: kindDesigns,
		roots: bothRoots},

	{subsection: "constraints", kind: kindCovenant, roots: bothRoots},
	{subsection: "constraints", groupID: "rules", kind: kindFiles, roots: []rootKind{rootInstallation},
		files: []string{"docs/covenant-evidence.md", "docs/project-rules.md"}},
	{subsection: "constraints", groupID: "rules", kind: kindFiles, roots: []rootKind{rootCheckout},
		files: []string{"development/project-rules-local.md"}, selfHostedOnly: true},

	{subsection: "open-questions", groupID: "registers", groupTitle: "Nearest living registers",
		kind: kindFiles, roots: []rootKind{rootStateRoot},
		files: []string{"memory/known-issues.md", "memory/proposal-drafts.md"}},
}
