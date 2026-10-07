package main

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/up"
)

type claimHolderReaders struct {
	Holder func(string) (lease.CurrentHolderView, error)
	Caller func(string, int64) (lease.ClassifyResult, error)
}

// prepareAgentClaim reuses only the original caller's authenticated owning
// session. Preparation does not grant authority: classification and the holder
// are read again before any claim decision.
func (inv *intentInvocation) prepareAgentClaim() (string, *up.Result, *intentResult) {
	root := inv.layout.InstallationRoot.Path()
	facts := inv.owners.dependencies.authorityFacts
	read := func() (lease.ClassifyResult, lease.CurrentHolderView, error) {
		classification, err := brainHumanWordClassificationWithFacts("claim", root, "", nil, facts)
		if err != nil {
			return classification, lease.CurrentHolderView{}, err
		}
		holder, err := lease.CurrentHolder(root)
		if err == lease.ErrLeaseAbsent {
			err = nil
		}
		return classification, holder, err
	}
	valid := func(c lease.ClassifyResult, h lease.CurrentHolderView) bool {
		return c.Class == lease.ClassMain && c.Holder && c.ClaimEpoch != nil && *c.ClaimEpoch > 0 &&
			c.MainId == h.MainId && *c.ClaimEpoch == h.ClaimEpoch && h.OwnerLineage != ""
	}
	refuse := func(err error) *intentResult {
		return &intentResult{Outcome: intentRefused, code: 1, Summary: "this session's authority could not be confirmed; nothing was claimed: " + err.Error(),
			next: inv.publicArgv("session", "start"), nextReason: "after repairing the reported authority cause; then repeat the claim"}
	}
	c, h, err := read()
	if err != nil {
		return "", nil, refuse(err)
	}
	if valid(c, h) {
		return h.OwnerLineage, nil, nil
	}
	// Custody identifies work owned by another process. Starting a session
	// for that work must never turn it into the checkout's writer.
	switch c.Class {
	case lease.ClassDelegate, lease.ClassSupervision, lease.ClassAdapterSupervisor, lease.ClassSteward:
		session := "the session that holds this checkout"
		if h.MainId != "" {
			session += " (" + h.MainId + ")"
		}
		return "", nil, &intentResult{Outcome: intentRefused, code: 1,
			Summary:  fmt.Sprintf("the original caller is %s and has no claim authority; nothing was claimed", c.Class),
			Decision: "make the claim from " + session}
	}
	prepared, problem := inv.prepareClaimSession(inv.claimLineage())
	if problem != nil {
		return "", nil, problem
	}
	if prepared.ExitCode() != 0 || prepared.Outcome == "stopped" {
		return "", &prepared, &intentResult{Outcome: intentRefused, code: 1, Summary: "session preparation ended " + prepared.Outcome + "; nothing was claimed",
			Decision: prepared.Remedy}
	}
	c, h, err = read()
	if err != nil {
		return "", &prepared, refuse(err)
	}
	if !valid(c, h) || prepared.Authority == "read-only" {
		if prepared.Authority == "read-only" || c.Class == lease.ClassMain && c.MainId != "" && c.MainId != h.MainId {
			return "", &prepared, &intentResult{Outcome: intentRefused, code: 1, Summary: fmt.Sprintf("another live session holds this checkout (%s); nothing was claimed", h.MainId),
				next: inv.publicArgv("session", "isolate"), nextReason: "then repeat the claim in the resulting writer session"}
		}
		return "", &prepared, refuse(fmt.Errorf("the original caller is %s without a positive owning epoch", c.Class))
	}
	return h.OwnerLineage, &prepared, nil
}

func (r claimHolderReaders) ReadHolder(root string) (lease.CurrentHolderView, error) {
	if r.Holder != nil {
		return r.Holder(root)
	}
	return lease.CurrentHolder(root)
}

func (r claimHolderReaders) Classify(root string, pid int64) (lease.ClassifyResult, error) {
	if r.Caller != nil {
		return r.Caller(root, pid)
	}
	return lease.ClassifyVerb(root, pid)
}

func (r claimHolderReaders) Facts(root string) (goal.ClaimHolderFacts, error) {
	h, err := r.ReadHolder(root)
	if err != nil {
		return goal.ClaimHolderFacts{}, err
	}
	c, err := r.Classify(root, h.Pid)
	if err != nil {
		return goal.ClaimHolderFacts{}, err
	}
	return goal.ClaimHolderFacts{Lineage: h.OwnerLineage, Epoch: h.ClaimEpoch,
		LiveMain: c.Class == lease.ClassMain && c.Holder && c.MainId == h.MainId && c.ClaimEpoch != nil && *c.ClaimEpoch == h.ClaimEpoch}, nil
}
