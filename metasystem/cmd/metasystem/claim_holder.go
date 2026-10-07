package main

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

type claimHolderReaders struct {
	Holder func(string) (lease.CurrentHolderView, error)
	Caller func(string, int64) (lease.ClassifyResult, error)
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
