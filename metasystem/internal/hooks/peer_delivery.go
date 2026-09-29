package hooks

// The shared offer of a peer message (batch-lane design D14-r3, R26; round
// D14D): the oldest message pending for this seat under the ownership rule,
// rendered after its fixed preface, offered only when the whole rendered
// text fits the room left in the field its runtime declared, and marked only
// after the response that carried it was emitted. The Stop path never calls
// it.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// peerSeparator joins a peer message to what the field already carries.
const peerSeparator = "\n\n"

// PeerOffer is one offered message: the text to place in the field, its id,
// and the line a hook prints on stderr when goal messages wait. The goals'
// claim locks taken for the ownership read are held until Release, so the
// read, the emission and the marker are ordered against a handover
// (D14D-01); a caller releases once its response is out, marked or not.
type PeerOffer struct {
	Text    string
	ID      string
	Waiting string

	mark    func() error
	release func()
}

// Mark records the offer as delivered to this seat; call it only after the
// response carrying Text was written without error.
func (o *PeerOffer) Mark() error {
	if o == nil || o.mark == nil {
		return errors.New("no peer message was offered")
	}
	return o.mark()
}

// Release gives back the claim locks the offer holds; safe to call twice.
func (o *PeerOffer) Release() {
	if o != nil && o.release != nil {
		o.release()
		o.release = nil
	}
}

// OfferPeerMessage reads what is pending for seat on the board under home
// and offers the oldest message when its rendered text is at most room
// bytes; ok is false when nothing is offered, and the offer (possibly
// carrying a Waiting line) must still be released. claims is asked for only
// when a goal mailbox holds a message this seat could receive; nil reads
// every goal message as nobody's.
func OfferPeerMessage(home, seat, lineage string, claims func() (map[string]string, error), event string, room int, now time.Time) (*PeerOffer, bool) {
	if claims == nil {
		claims = func() (map[string]string, error) { return map[string]string{}, nil }
	}
	inbox, err := board.Pending(home, seat, claims, now)
	offer := &PeerOffer{release: inbox.Release}
	if err != nil {
		offer.Waiting = "peer messages wait: " + err.Error()
		return offer, false
	}
	if inbox.GoalWaiting > 0 {
		reason := "a handover of their goal is in progress"
		if inbox.Unreadable != "" {
			reason = "the ledger is unreadable (" + inbox.Unreadable + ")"
		}
		offer.Waiting = fmt.Sprintf("%d goal messages wait: %s", inbox.GoalWaiting, reason)
	}
	if len(inbox.Messages) == 0 {
		return offer, false
	}
	message := inbox.Messages[0]
	text := board.Render(message, now, time.Local)
	if len(text) > room {
		return offer, false
	}
	offer.Text, offer.ID = text, message.ID
	offer.mark = func() error { return board.Mark(message, seat, lineage, event, now) }
	return offer, true
}

// peerClaims turns the Ops answer into the claims the ownership rule reads:
// {"goal":"machine"}, or an error naming why the ledger is unreadable.
func peerClaims(ops Ops, repo string) func() (map[string]string, error) {
	return func() (map[string]string, error) {
		out, status := ops.PeerClaims(repo)
		out = trimNewlines(out)
		if status != 0 {
			if out == "" {
				out = "the accepted ledger cannot be read"
			}
			return nil, errors.New(out)
		}
		claims := map[string]string{}
		if err := json.Unmarshal([]byte(out), &claims); err != nil {
			return nil, fmt.Errorf("the ledger's claims are malformed: %w", err)
		}
		return claims, nil
	}
}

// preparePeer offers the oldest pending peer message at session start: only
// when the packet goes to the model's declared field (a screen or no-context
// outcome gets none), and only when the packet, the separator and the whole
// rendered message fit the field's byte bound. The packet is never cut; a
// message that does not fit waits for the next tool call.
func (s *startRun) preparePeer() {
	if s.contextKind != "channel" || s.contextPayload == "" || s.contextField == "" {
		return
	}
	home, err := board.HomeWith(s.inv.Lookup)
	if err != nil || !board.HasMessages(home) {
		return
	}
	seat, status := s.ops.PeerSeat(s.repo)
	s.checkpoint()
	if seat = trimNewlines(seat); status != 0 || !board.SafeName(seat) {
		return
	}
	now := time.Now()
	if s.inv.Now != nil {
		now = s.inv.Now()
	}
	offer, ok := OfferPeerMessage(home, seat, s.session, peerClaims(s.ops, s.repo), "start", s.contextBytes-len(s.contextPayload)-len(peerSeparator), now)
	s.peer = offer
	if offer.Waiting != "" {
		_ = writeLine(s.inv.Stderr, "Metasystem peer messages: "+offer.Waiting)
	}
	if ok {
		s.contextPayload += peerSeparator + offer.Text
		s.peerOffered = true
	}
}

// markPeer is the start's bookkeeping after its context was published: the
// offered message is marked only when the emitted response carried it.
func (s *startRun) markPeer() {
	if !s.peerOffered || !strings.Contains(s.contextPayload, s.peer.Text) {
		return
	}
	if err := s.peer.Mark(); err != nil {
		_ = writeLine(s.inv.Stderr, "Metasystem peer message "+s.peer.ID+" was delivered but not marked ("+err.Error()+"); it is offered again at the next start or tool call.")
	}
}
