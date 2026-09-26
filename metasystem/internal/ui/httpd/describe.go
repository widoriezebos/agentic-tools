package httpd

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/manifest"
)

// What this interface lets a human do, described where the routes that do it
// live.
//
// The acts are the write routes of this package and nowhere else, so this is
// where they are named. What each row carries beside its name is the GENERAL
// requirement: what any caller must be for the route to be admitted at all.
// It is not this request's eligibility — whether the human at this keyboard
// may act right now is answered when they ask, by mayAct, with both proofs'
// own words — and running the two together is how a description starts
// promising a human something the server will refuse.
//
// The Project Partner performs none of them. Its process has no session, no
// cookie and no way to obtain one; a configured Partner is also what closes
// the boot proof's path to the ledger's acts, which is why the ledger row
// below says so.

const (
	// ledgerHand is what the ledger's four acts need. It is mayAct's rule in
	// one sentence, and mayAct is what enforces it.
	ledgerHand = "a signed-in human; on a seat that has named no Project Partner, the boot proof of the terminal that started the server answers instead"
	// checkoutHand is what the project's writes need. They edit files of this
	// checkout on a loopback server and record no authority in the ledger, so
	// they ask for no proof of one.
	checkoutHand = "a request from this browser to this loopback server; the write lands in the checkout and records no ledger authority"
	// conversationHand is what the Partner's own two writes need. They carry
	// no authority at all: one admits a question, one stops the answer.
	conversationHand = "a request from this browser; it carries no authority of any kind and changes nothing but the conversation"
	// notepadHand is what the notepad's own read and three acts need. It is
	// the checkout writes' policy exactly — a POST from this browser to this
	// loopback server, judged for its host, its site and its origin, carrying
	// no proof of ledger authority — said about where the write actually
	// lands. A sticky is not a file of the checkout and is deliberately not
	// kept in one, so promising a human that it lands there would be this
	// table telling them the one thing about their own notes that is not
	// true.
	notepadHand = "a request from this browser to this loopback server; the write lands in this account's own notepad, outside every checkout, and records no ledger authority"
)

// Acts is every act this interface offers, with the hand each one needs.
func Acts() []manifest.Act {
	return []manifest.Act{
		{ID: routeOpen, Title: "Open a goal", Requires: ledgerHand,
			Does: "Writes a new goal into the ledger at intake, under origin human, with its risk answers and the tier they derive."},
		{ID: routeApprove, Title: "Approve a goal", Requires: ledgerHand,
			Does: "Authorises a goal so a seat may claim it, with the budget the approval carries."},
		{ID: routeWithdraw, Title: "Withdraw a goal's approval", Requires: ledgerHand,
			Does: "Takes back an authorisation, with the reason, so no seat claims the goal."},
		{ID: routePriority, Title: "Set a goal's priority", Requires: ledgerHand,
			Does: "Places a goal in a priority band at a position, renumbering the band."},
		{ID: routeBlock, Title: "Say a goal waits for another", Requires: ledgerHand,
			Does: "Records that one goal waits for another; the waiting goal parks unless the goal it waits for is already done."},
		{ID: routeUnblock, Title: "Say a goal no longer waits for another", Requires: ledgerHand,
			Does: "Removes one such edge; the park lifts only when the dependency created it and every remaining goal it waits for is done."},
		{ID: routePark, Title: "Park a goal", Requires: ledgerHand,
			Does: "Pauses a goal with the reason a human gave, so it leaves the queue until somebody returns it."},
		{ID: routeUnpark, Title: "Return a parked goal to the queue", Requires: ledgerHand,
			Does: "Lifts a park, returning the goal to approved where its approval still stands and to queued otherwise."},
		{ID: routeCreateRecord, Title: "Write a record", Requires: checkoutHand,
			Does: "Creates one decision, design, doctrine or intent record in the home its kind names."},
		{ID: routeRecordStatus, Title: "Set a record's status", Requires: checkoutHand,
			Does: "Moves one record between draft, accepted, superseded and done."},
		{ID: routeRecordGoals, Title: "Name a goal on a record", Requires: checkoutHand,
			Does: "Adds one ledger goal to a record's own head."},
		{ID: routeAskQuestion, Title: "Ask an open question", Requires: checkoutHand,
			Does: "Appends one row to the questions register."},
		{ID: routeQuestionStatus, Title: "Settle an open question", Requires: checkoutHand,
			Does: "Moves one question between open, answered and withdrawn."},
		{ID: routeEditDocument, Title: "Edit a document", Requires: checkoutHand,
			Does: "Saves one document of the checkout against the revision it was opened at."},
		{ID: routePreviewSource, Title: "Preview what is being typed", Requires: checkoutHand,
			Does: "Renders unsaved text through the reader's own parser. It opens nothing and writes nothing."},
		{ID: routeStickies, Title: "Read your stickies", Requires: notepadHand,
			Does: "Answers this human's own notepad: every sticky, open ones newest first, with how many are open and how many are done."},
		{ID: routeAddSticky, Title: "Write a sticky", Requires: notepadHand,
			Does: "Writes one reminder, with what it is about, and answers the whole notepad."},
		{ID: routeEditSticky, Title: "Change a sticky", Requires: notepadHand,
			Does: "Rewrites one sticky's text or what it is about, or marks it done or open again, and answers the whole notepad."},
		{ID: routeRemoveSticky, Title: "Remove a sticky", Requires: notepadHand,
			Does: "Takes one sticky off this human's notepad for good, and answers the rest."},
		{ID: routeSignIn, Title: "Sign in", Requires: "this seat's one-time code",
			Does: "Opens a browser session in this human's name, which is what the ledger's acts publish under."},
		{ID: routeSignOut, Title: "Sign out", Requires: "a live browser session",
			Does: "Ends this browser session."},
		{ID: routePartnerTurn, Title: "Ask the Project Partner", Requires: conversationHand,
			Does: "Admits one question to the Partner and streams the answer back."},
		{ID: routePartnerStop, Title: "Stop the Partner", Requires: conversationHand,
			Does: "Cancels the running answer and keeps what it had said."},
		{ID: routePartnerSeeing, Title: "Show what the Partner will see", Requires: conversationHand,
			Does: "Composes the block the next question would carry, without sending anything."},
		// Starting a sitting asks for the checkout's own hand rather than the
		// conversation's, because it may write: a sitting started on a new draft
		// creates that record in the checkout before the conversation is marked.
		// Ending one changes nothing but the conversation.
		{ID: routePartnerSitting, Title: "Start a sitting", Requires: checkoutHand,
			Does: "Opens a working conversation on one record — an existing one, or a draft created now in the home its kind names — and asks the Partner what the records already hold about it."},
		{ID: routePartnerClose, Title: "Draft the sitting's outcome", Requires: conversationHand,
			Does: "Asks the Partner for the closing deposit — the outcome as decided, the constraints, the open questions with their consequences, and what the table holds. It ends nothing: the sitting stands until the human has recorded the outcome or left without it."},
		{ID: routePartnerRise, Title: "End the sitting", Requires: conversationHand,
			Does: "Takes the sitting off the conversation. What was recorded stays in the record, which is the whole of what a sitting leaves behind."},
	}
}
