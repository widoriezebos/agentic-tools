package main

import (
	"log"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The walkthrough's shaping room (g1-s67): a design record with sections to put
// on the desk, one source file of the checkout the Today walk reads as it
// stands, a Partner that opens the sitting with anchored words and walks
// through Today by putting that file on the desk, and — behind a flag — a
// sitting marked on the human's ordinary conversation before every sitting had
// a conversation of its own (D6, Astra S67-04).

const (
	// shapingRecord is the design a sitting is started on in the walkthrough.
	shapingRecord = "plans/designs/sessions.md"
	// shapingFile is the checkout's own code the Today walk puts on the desk.
	shapingFile = "internal/session/owner.go"
	// beforeRoomsRecord is the design the pre-D16 sitting stands on.
	beforeRoomsRecord = "plans/designs/limits.md"
)

const shapingText = `# g1-s66: session limits

- Kind: design
- Id: design-sessions
- Status: draft

## 1. What exists and binds

A session is taken when a human signs in and lasts twelve hours from then. The
owner holds one lock while it writes the session file.

## 2. What you want

A session that ends when nobody has touched the page for a working day, and
never while somebody is typing.

## 3. Decisions

- D1. The limit counts from the last activity, not from sign-in.
- D2. A sleeping laptop is not activity.

## Facts

## Proposals

## Decisions

## Open questions
`

const beforeRoomsText = `# Session limits, the first draft

- Kind: design
- Id: design-limits
- Status: draft

## 1. What exists

A session lasts twelve hours from sign-in.

## Facts

## Proposals

## Decisions

## Open questions
`

const shapingCode = `package session

import (
	"sync"
	"time"
)

// owner writes the session file under one lock.
type owner struct {
	mu sync.Mutex
}

// begin takes the lock, stamps the session and writes it.
func (o *owner) begin(human string, now time.Time) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	session := Session{Human: human, At: now}
	// The limit is counted from here, which is sign-in.
	session.Until = now.Add(12 * time.Hour)
	if err := write(session); err != nil {
		return err
	}
	return nil
}

// touch is called on every request, and moves nothing yet.
func (o *owner) touch(now time.Time) {}
`

// The phrases of the shaping sitting's fixed requests the canned answers are
// narrowed to.
const (
	shapingOpening = "Open this sitting on " + shapingRecord
	shapingToday   = "Walk me through Today for the sitting on"
)

// shapingAnswers are the shaping sitting's canned answers: its opening, every
// claim about today anchored in the checkout, and the Today walk.
var shapingAnswers = []fakeacp.Answer{
	{When: shapingOpening, Chunks: []string{
		"## What the records hold\n\nThe design records two decisions, D1 and D2, and no open question yet. " +
			"Nothing recorded says what the twelve-hour limit protects.\n\n",
		"## What the application does today\n\nThe limit is stamped once, at sign-in, in " +
			"`internal/session/owner.go:13-24`, and `internal/session/owner.go:27` moves nothing on a request.\n\n",
		"I have offered a fact, a decision and a case for the record; they are yours to record, decide or leave open.\n",
	}},
	{When: shapingToday, Chunks: []string{
		"Today, in the order I am putting it on the desk.\n\n",
		"The session is begun in `internal/session/owner.go:13-24`: the lock is taken, the session is stamped, and " +
			"the limit is counted from sign-in at line 19.\n\n",
		"A request calls `internal/session/owner.go:27`, which moves nothing, so a page used all day still ends at " +
			"twelve hours.\n",
	}},
}

// shapingReads are the Today walk's desk items.
var shapingReads = []fakeacp.Read{
	{
		When:  shapingToday,
		Name:  "mcp__" + uitools.ServerName + "__" + uitools.OpPresent,
		Title: "present(source)",
		Result: uitools.PresentedLine + "\n" + uitools.PresentHeader + "source\n" + uitools.PresentPath + shapingFile +
			"\n" + uitools.PresentFrom + "13\n" + uitools.PresentTo + "24\n",
	},
}

// plantBeforeRooms marks the human's ordinary conversation with a shaping
// sitting on beforeRoomsRecord and a transcript, as a sitting started before
// D16 left it: no record-keyed files beside it (g1-s67 D6).
func plantBeforeRooms(conversations, human string) {
	at := time.Now().UTC().Add(-26 * time.Hour)
	stamp := at.Format(time.RFC3339)
	ordinary, err := partner.OpenConversation(conversations, human)
	if err != nil {
		log.Fatalf("cannot open the ordinary conversation to plant: %v", err)
	}
	subject := partner.Subject{Kind: partner.SubjectRecord, ID: beforeRoomsRecord, Title: "Session limits, the first draft"}
	if err := ordinary.Sit(partner.Sitting{Subject: subject, Purpose: partner.PurposeShapeDesign, StartedAt: stamp}, at); err != nil {
		log.Fatalf("cannot plant the sitting: %v", err)
	}
	for _, message := range []partner.Message{
		{ID: "before-1", Turn: "before-t1", Role: partner.RoleHuman, At: stamp,
			Text: "What does the twelve-hour limit protect, and is it written down anywhere?"},
		{ID: "before-2", Turn: "before-t1", Role: partner.RolePartner, At: stamp, Outcome: partner.OutcomeComplete,
			Text: strings.Join([]string{
				"Nothing recorded says what it protects.",
				"The limit is stamped at sign-in in `internal/session/owner.go:19`.",
			}, " ")},
	} {
		if err := ordinary.Append(message); err != nil {
			log.Fatalf("cannot plant the transcript: %v", err)
		}
	}
	log.Printf("Project Partner: a sitting on %s planted on %s's ordinary conversation, as before D16", beforeRoomsRecord, human)
}
