package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/lifecycle"
)

// uiSeat is one other machine of this computer and its interface's roots.
type uiSeat struct {
	Name     string
	Checkout string
	Roots    lifecycle.Roots
}

// uiSeatInventory is the other machines of this computer as machine list
// names them; Problems says why the list may be incomplete.
type uiSeatInventory struct {
	This     string
	Seats    []uiSeat
	Problems []string
}

// uiSeatView is one other machine's interface as a verb names it.
type uiSeatView struct {
	Machine  string          `json:"machine"`
	Checkout string          `json:"checkout"`
	State    lifecycle.State `json:"state"`
	Pid      int64           `json:"pid,omitempty"`
	Address  string          `json:"address,omitempty"`
}

func (inv *intentInvocation) uiOtherSeats(layout stateroot.Layout) uiSeatInventory {
	return uiSeatInventory{}
}
