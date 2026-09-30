package main

import (
	"fmt"
	"io"
	"sync"
)

// admissionNotice is what a helm or power-of-attorney admission tells the
// person about the act it admitted: one line, and the detail --verbose adds.
type admissionNotice struct {
	w            io.Writer
	line, detail string
}

// admissionNotices holds the admission notices of the public command this
// process runs until its outcome is known ("Messages a Person Reads": lines
// never contradict each other). An act that proceeds prints its notice before
// its result; a refused act prints the refusal alone, the notice only with
// --verbose. Outside a held command, or when nothing holds the board, a
// notice is printed at once.
type admissionNotices struct {
	mu      sync.Mutex
	armed   bool
	holding int
	pending []admissionNotice
}

// processAdmissionNotices is the board of this process's command; main arms
// it. In-process tests leave it unarmed, so a notice prints at once there
// and parallel tests never share a pending notice.
var processAdmissionNotices = &admissionNotices{}

func (n *admissionNotices) arm() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.armed = true
}

// say prints notice, or keeps it while a command holds the board.
func (n *admissionNotices) say(notice admissionNotice) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.holding == 0 {
		fmt.Fprintln(notice.w, notice.line)
		return
	}
	n.pending = append(n.pending, notice)
}

// hold keeps notices until the outermost holder settles or releases; an
// unarmed board holds nothing and hold reports false.
func (n *admissionNotices) hold() (func(), bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if !n.armed {
		return func() {}, false
	}
	n.holding++
	return func() {
		n.mu.Lock()
		defer n.mu.Unlock()
		n.holding--
		if n.holding == 0 {
			// The command ended without a rendered outcome: the notice
			// still tells what was admitted.
			for _, notice := range n.pending {
				fmt.Fprintln(notice.w, notice.line)
			}
			n.pending = nil
		}
	}, true
}

// settle ends the wait for the outermost command's outcome: an act that
// proceeds prints each notice (and its detail when verbose); a refused act
// prints none and gets them back as detail lines.
func (n *admissionNotices) settle(refused, verbose bool) []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.holding != 1 {
		return nil
	}
	pending := n.pending
	n.pending = nil
	var details []string
	for _, notice := range pending {
		if refused {
			details = append(details, "not done, although: "+notice.line, notice.detail)
			continue
		}
		fmt.Fprintln(notice.w, notice.line)
		if verbose && notice.detail != "" {
			fmt.Fprintln(notice.w, notice.detail)
		}
	}
	return details
}
